package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/internal/data/metadata/base"
	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	roledata "github.com/telark/telark/internal/data/resources/role"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
	userdata "github.com/telark/telark/internal/data/resources/user"
	"github.com/telark/telark/internal/kcore/k8sclient"
	bootstrapeps "github.com/telark/telark/internal/rest/endpoints/bootstrap"
	"github.com/telark/telark/internal/rest/router"
	"github.com/telark/telark/internal/x-ware/async"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
	exprdb "github.com/telark/telark/services/exporter/internal/redis"
	notifstorage "github.com/telark/telark/services/exporter/internal/redis/notifications"
	"github.com/telark/telark/services/exporter/internal/routes"
	notiftypes "github.com/telark/telark/services/exporter/internal/types/notifications"
	"github.com/telark/telark/services/exporter/internal/utils/performance"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

const (
	pinnedID     = "u-00001-0000-0001"
	otherID      = "u-00002-0000-0002"
	adminID      = "u-00003-0000-0003"
	readerID     = "u-00004-0000-0004"
	suspendedID  = "u-00005-0000-0005"
	pinnedEmail  = "admin@example.com"
	otherEmail   = "test@example.com"
	adminRoleID  = "r-00000-0000-0001"
	readerRoleID = "r-00000-0000-0004"
	listSuffix   = "List"
	firstVersion = "1"
	verbPatch    = "patch"
	keyRV        = "resourceVersion"
	keyMetadata  = "metadata"
	noticeLimit  = 50
	basePath     = "/api/v1/bootstrap"
	requestsPath = basePath + "/requests"
	usersPath    = "/api/v1/users"
	passkeysPath = "/api/v1/internal/auth/passkeys"
	noticesPath  = "/api/v1/notifications"
	configPath   = "/api/v1/config"
	pathSep      = "/"
	base36       = 36
)

var (
	configGVR = gvrOf(v1alpha1.TelarkConfigMetadata)
	internal  = xauthz.Identity{Internal: true}
	errStale  = errors.New("the object has been modified")
)

func gvrOf(md base.Metadata) schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: md.BaseGroup, Version: md.Version, Resource: md.Plural}
}

type seed struct {
	md  base.Metadata
	obj *unstructured.Unstructured
}

func cr(t *testing.T, md base.Metadata, name string, record any) seed {
	t.Helper()
	spec, err := sharedutils.StructToSpecMap(record)
	if err != nil {
		t.Fatal(err)
	}
	obj := sharedutils.ConvertToCRDTemplate(md, name, spec)
	obj.SetNamespace(md.Namespace)
	return seed{md: md, obj: obj}
}

type account struct {
	id, email, phase string
	bootstrap        bool
	roles            []string
	lastLogin        *string
	created          time.Time
}

func (a account) seed(t *testing.T) seed {
	t.Helper()
	user := &userdata.User{ID: a.id, Username: a.id, Email: a.email, Bootstrap: a.bootstrap,
		Status:     userdata.UserStatus{Phase: a.phase, LastLoginAt: a.lastLogin},
		Identities: []*userdata.UserIdentity{{Provider: constants.IdentityProviderPasskey, Subject: a.id}}}
	for i := range a.roles {
		user.RoleRefs = append(user.RoleRefs, &a.roles[i])
	}
	s := cr(t, v1alpha1.UserMetadata, a.id, user)
	s.obj.SetCreationTimestamp(metav1.NewTime(a.created))
	return s
}

func active(id, email string, bootstrap bool, roles ...string) account {
	return account{id: id, email: email, phase: string(userdata.AccountPhaseActive), bootstrap: bootstrap, roles: roles}
}

func passkey(t *testing.T, userID string, created time.Time) seed {
	t.Helper()
	name := userID + "-passkey-" + strconv.FormatInt(created.UnixNano(), base36)
	s := cr(t, v1alpha1.PasskeyMetadata, name, map[string]any{constants.UserIDParam: userID, "credentialId": name})
	s.obj.SetCreationTimestamp(metav1.NewTime(created))
	return s
}

func session(t *testing.T, userID string) seed {
	t.Helper()
	return cr(t, v1alpha1.SessionMetadata, "session-"+userID, map[string]any{constants.UserIDParam: userID})
}

// An Admin on ALL, a ReadOnly user and a suspended Admin: only the first hears of a request besides the pinned account.
func directory(t *testing.T) []seed {
	t.Helper()
	role := func(id, scope string, level roledata.PermissionLevel) seed {
		return cr(t, v1alpha1.AccessRoleMetadata, id, roledata.AccessRole{ID: id, Name: id, Status: roledata.RoleStatusActive,
			ScopesAndPermissions: []roledata.ScopeAndPermissions{{Scope: scope, Level: level}}})
	}
	suspended := active(suspendedID, "suspended@example.com", false, adminRoleID)
	suspended.phase = string(userdata.AccountPhaseSuspended)
	return []seed{
		role(adminRoleID, roledata.ScopeAll, roledata.PermissionLevelAdmin),
		role(readerRoleID, roledata.ScopeAll, roledata.PermissionLevelReadOnly),
		active(adminID, "admins@example.com", false, adminRoleID).seed(t),
		active(readerID, "readers@example.com", false, readerRoleID).seed(t),
		suspended.seed(t),
	}
}

// The bootstrap account pinned an hour ago, with these requests, newest first.
func pinned(requests ...telarkconfig.BootstrapRequest) *telarkconfig.BootstrapStatus {
	return &telarkconfig.BootstrapStatus{UserID: pinnedID, Email: pinnedEmail, PinnedAt: stamp(time.Now().Add(-time.Hour)), Requests: requests}
}

func configWith(t *testing.T, status *telarkconfig.BootstrapStatus) seed {
	t.Helper()
	obj := sharedutils.ConvertToCRDTemplate(v1alpha1.TelarkConfigMetadata, constants.TelarkConfigResourceName, map[string]any{})
	obj.SetNamespace(v1alpha1.TelarkConfigMetadata.Namespace)
	obj.SetResourceVersion(firstVersion)
	if status != nil {
		raw, err := json.Marshal(status)
		if err != nil {
			t.Fatal(err)
		}
		var stored map[string]any
		if err := json.Unmarshal(raw, &stored); err != nil {
			t.Fatal(err)
		}
		obj.Object[constants.FieldStatus] = map[string]any{telarkconfig.FieldBootstrap: stored}
	}
	return seed{md: v1alpha1.TelarkConfigMetadata, obj: obj}
}

func alertsOff(t *testing.T, config seed) seed {
	t.Helper()
	path := []string{constants.SpecField, telarkconfig.FieldBreakGlassAlerts, telarkconfig.FieldNotifyAdmins}
	if err := unstructured.SetNestedField(config.obj.Object, false, path...); err != nil {
		t.Fatal(err)
	}
	return config
}

func statusAs(t *testing.T, identity xauthz.Identity) bootstrapeps.StatusResult {
	t.Helper()
	w := serveAs(t, identity, http.MethodGet, basePath, constants.EmptyString)
	if w.Code != http.StatusOK {
		t.Fatalf("GET bootstrap = %d %s", w.Code, w.Body.String())
	}
	var envelope struct {
		Data bootstrapeps.StatusResult `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data
}

// The fake would guess plurals from kinds, so objects go in through the resource client.
func installFake(t *testing.T, seeds ...seed) *dynamicfake.FakeDynamicClient {
	t.Helper()
	listKinds := map[schema.GroupVersionResource]string{}
	for _, md := range []base.Metadata{
		v1alpha1.TelarkConfigMetadata, v1alpha1.UserMetadata, v1alpha1.GroupMetadata, v1alpha1.AccessRoleMetadata,
		v1alpha1.SessionMetadata, v1alpha1.PasskeyMetadata,
	} {
		listKinds[gvrOf(md)] = md.Kind + listSuffix
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds)
	for _, s := range seeds {
		create(t, client, s)
	}
	client.PrependReactor(verbPatch, v1alpha1.TelarkConfigMetadata.Plural, compareAndSwap(client))
	k8sclient.SetDynamicClient(client)
	t.Cleanup(func() { k8sclient.SetDynamicClient(nil) })
	// Notices go out on the async pool: a test's own land before the next test points the store elsewhere.
	t.Cleanup(func() {
		async.Drain()
		async.Init()
	})
	return client
}

func TestMain(m *testing.M) {
	async.Init()
	m.Run()
}

// Runs before each TelarkConfig patch, outside the fake's lock.
type configPatchHook struct {
	dynamic.Interface
	before func()
}

func (c configPatchHook) Resource(gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	resource := c.Interface.Resource(gvr)
	if gvr != gvrOf(v1alpha1.TelarkConfigMetadata) {
		return resource
	}
	return hookedResource{NamespaceableResourceInterface: resource, before: c.before}
}

type hookedResource struct {
	dynamic.NamespaceableResourceInterface
	before func()
}

func (h hookedResource) Namespace(namespace string) dynamic.ResourceInterface {
	return hookedNamespace{ResourceInterface: h.NamespaceableResourceInterface.Namespace(namespace), before: h.before}
}

type hookedNamespace struct {
	dynamic.ResourceInterface
	before func()
}

func (h hookedNamespace) Patch(
	ctx context.Context, name string, pt types.PatchType, data []byte, options metav1.PatchOptions, subresources ...string,
) (*unstructured.Unstructured, error) {
	h.before()
	return h.ResourceInterface.Patch(ctx, name, pt, data, options, subresources...)
}

// The fake applies a patch whatever resourceVersion it names, so this does what the API server does: a patch that
// names a stale version conflicts, and each write moves the version on.
func compareAndSwap(client *dynamicfake.FakeDynamicClient) k8stesting.ReactionFunc {
	apply := k8stesting.ObjectReaction(client.Tracker())
	return func(action k8stesting.Action) (bool, runtime.Object, error) {
		patch, isPatch := action.(k8stesting.PatchActionImpl)
		if !isPatch {
			return false, nil, nil
		}
		current, err := client.Tracker().Get(configGVR, patch.GetNamespace(), patch.GetName())
		if err != nil {
			return true, nil, err
		}
		version := current.(*unstructured.Unstructured).GetResourceVersion()
		var body map[string]any
		if err := json.Unmarshal(patch.GetPatch(), &body); err != nil {
			return true, nil, err
		}
		meta, hasMeta := body[keyMetadata].(map[string]any)
		if !hasMeta {
			meta = map[string]any{}
			body[keyMetadata] = meta
		}
		if sent, named := meta[keyRV].(string); named && sent != version {
			return true, nil, apierrors.NewConflict(configGVR.GroupResource(), patch.GetName(), errStale)
		}
		number, err := strconv.Atoi(version)
		if err != nil {
			return true, nil, err
		}
		meta[keyRV] = strconv.Itoa(number + constants.DefaultIncrementValue)
		if patch.Patch, err = json.Marshal(body); err != nil {
			return true, nil, err
		}
		return apply(patch)
	}
}

// An older CRD has no status.bootstrap: the API server drops it from every write and still answers 200.
func pruneBootstrap(t *testing.T, client *dynamicfake.FakeDynamicClient) {
	t.Helper()
	cas := compareAndSwap(client)
	client.PrependReactor(verbPatch, v1alpha1.TelarkConfigMetadata.Plural, func(action k8stesting.Action) (bool, runtime.Object, error) {
		handled, obj, err := cas(action)
		if err != nil || !handled {
			return handled, obj, err
		}
		config := obj.(*unstructured.Unstructured).DeepCopy()
		unstructured.RemoveNestedField(config.Object, constants.FieldStatus, telarkconfig.FieldBootstrap)
		return true, config, client.Tracker().Update(configGVR, config, config.GetNamespace())
	})
}

// Replaces the stored status as a cluster admin would, outside the exporter.
func setStatus(t *testing.T, client *dynamicfake.FakeDynamicClient, status *telarkconfig.BootstrapStatus) {
	t.Helper()
	current, err := client.Tracker().Get(configGVR, v1alpha1.TelarkConfigMetadata.Namespace, constants.TelarkConfigResourceName)
	if err != nil {
		t.Fatal(err)
	}
	config := current.(*unstructured.Unstructured).DeepCopy()
	raw, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	var stored map[string]any
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	if err := unstructured.SetNestedMap(config.Object, stored, constants.FieldStatus, telarkconfig.FieldBootstrap); err != nil {
		t.Fatal(err)
	}
	if err := client.Tracker().Update(configGVR, config, config.GetNamespace()); err != nil {
		t.Fatal(err)
	}
}

func storedStatus(t *testing.T, client *dynamicfake.FakeDynamicClient) *telarkconfig.BootstrapStatus {
	t.Helper()
	obj, err := client.Resource(configGVR).Namespace(v1alpha1.TelarkConfigMetadata.Namespace).
		Get(context.Background(), constants.TelarkConfigResourceName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var status telarkconfig.BootstrapStatus
	stored, _, err := unstructured.NestedMap(obj.Object, constants.FieldStatus, telarkconfig.FieldBootstrap)
	if err == nil {
		raw, _ := json.Marshal(stored)
		err = json.Unmarshal(raw, &status)
	}
	if err != nil {
		t.Fatal(err)
	}
	return &status
}

func storedUser(t *testing.T, client *dynamicfake.FakeDynamicClient, id string) *userdata.User {
	t.Helper()
	obj, err := client.Resource(gvrOf(v1alpha1.UserMetadata)).Namespace(v1alpha1.UserMetadata.Namespace).
		Get(context.Background(), id, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	user, err := sharedutils.SpecToStruct[userdata.User](obj)
	if err != nil {
		t.Fatal(err)
	}
	return user
}

// The pinned account's records of a kind.
func countOf(t *testing.T, client *dynamicfake.FakeDynamicClient, md base.Metadata) int {
	t.Helper()
	list, err := client.Resource(gvrOf(md)).Namespace(md.Namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	count := constants.DefaultInitValue
	for _, item := range list.Items {
		if owner, _, _ := unstructured.NestedString(item.Object, constants.SpecField, constants.UserIDParam); owner == pinnedID {
			count++
		}
	}
	return count
}

// Notices land in an embedded Redis, emitted on the async pool.
func withNotices(t *testing.T) {
	t.Helper()
	exprdb.Set(redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()}))
	t.Cleanup(func() { exprdb.Set(nil) })
}

func notices(t *testing.T, userID string) []notiftypes.Notification {
	t.Helper()
	async.Drain()
	async.Init()
	storage, err := notifstorage.NewStorage()
	if err != nil {
		t.Fatal(err)
	}
	list, err := storage.List(context.Background(), userID, noticeLimit, constants.EmptyString)
	if err != nil {
		t.Fatal(err)
	}
	return list.Items
}

func serveAs(t *testing.T, identity xauthz.Identity, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	return serve(t, r.WithContext(xauthz.WithIdentity(r.Context(), identity)))
}

func serve(t *testing.T, r *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	optimizer := performance.NewOptimizer(redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()}))
	t.Cleanup(optimizer.Close)
	w := httptest.NewRecorder()
	router.NewRouter(routes.InitRoutes(optimizer, nil)).ServeHTTP(w, r)
	return w
}

func newOptimizer(t *testing.T) *performance.Optimizer {
	t.Helper()
	optimizer := performance.NewOptimizer(redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()}))
	t.Cleanup(optimizer.Close)
	return optimizer
}

// Auth's registration finish saves a passkey through this route, naming the account in a header.
func addPasskey(t *testing.T, userID string) {
	t.Helper()
	body := `{"credentialId":"cred-` + userID + `-` + strconv.FormatInt(time.Now().UnixNano(), base36) +
		`","publicKey":"AQID","deviceName":"laptop","deviceType":"platform"}`
	r := httptest.NewRequest(http.MethodPost, passkeysPath, strings.NewReader(body))
	r.Header.Set(constants.HeaderUserID, userID)
	if w := serve(t, r.WithContext(xauthz.WithIdentity(r.Context(), internal))); w.Code != http.StatusOK {
		t.Fatalf("passkey for %s = %d %s", userID, w.Code, w.Body.String())
	}
}

func create(t *testing.T, client *dynamicfake.FakeDynamicClient, s seed) {
	t.Helper()
	if _, err := client.Resource(gvrOf(s.md)).Namespace(s.md.Namespace).Create(context.Background(), s.obj, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
}

type result struct {
	Request  telarkconfig.BootstrapRequest `json:"request"`
	Eligible bool                          `json:"eligible"`
	Enrolled bool                          `json:"enrolled"`
}

// Calls a request route as the service and decodes the request it answered with.
func call(t *testing.T, path, body string, want int) result {
	t.Helper()
	w := serveAs(t, internal, http.MethodPost, path, body)
	if w.Code != want {
		t.Fatalf("POST %s = %d %s, want %d", path, w.Code, w.Body.String(), want)
	}
	var envelope struct {
		Data result `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data
}

func open(t *testing.T, email string, want int) result {
	t.Helper()
	return call(t, requestsPath, `{"email":"`+email+`"}`, want)
}

func act(t *testing.T, id, action, body string, want int) result {
	t.Helper()
	return call(t, requestsPath+pathSep+id+pathSep+action, body, want)
}

func recovery(state string, opened time.Time) telarkconfig.BootstrapRequest {
	return telarkconfig.BootstrapRequest{ID: requestID, Kind: telarkconfig.BootstrapKindRecover, State: state, OpenedAt: stamp(opened)}
}

func stamp(at time.Time) string {
	return at.UTC().Format(time.RFC3339)
}
