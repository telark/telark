package testutil

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	forbiddenMessage = `deployments.apps is forbidden: User "test" cannot list resource "deployments"`
	forbiddenFormat  = "%s is forbidden for this test"
	refusedFormat    = "%s is refused for this test"
	requestFormat    = "%s %s/%s"
	notFoundFormat   = "%s %q not found"
	contentType      = "Content-Type"
	jsonType         = "application/json"
	testToken        = "test-token"
	kubeconfigName   = "kubeconfig"
	versionPath      = "/version"
	corePrefix       = "api"
	groupPrefix      = "apis"
	namespacesPath   = "namespaces"
	listSuffix       = "List"
	selectorParam    = "labelSelector"
	watchParam       = "watch"
	reviewsResource  = "selfsubjectaccessreviews"
	whoAmIResource   = "selfsubjectreviews"
	whoAmIVersion    = "authentication.k8s.io/v1"
	whoAmIKind       = "SelfSubjectReview"
	DefaultUser      = "ops-admin"
	pathSeparator    = "/"
	groupSeparator   = "."
	coreSkip         = 2
	groupSkip        = 3
	first            = 0
	second           = 1
	third            = 2
	DefaultVersion   = "v1.34.0"
)

// The fake answers like the real API server, lists filtered by namespace and label selector and a 404 for a
// missing name, so a wrong path or selector fails the tests.
func APIServer(t *testing.T, objects ...runtime.Object) string {
	t.Helper()
	return ClusterServer(t, Cluster{Objects: objects})
}

func ClusterServer(t *testing.T, cluster Cluster) string {
	t.Helper()
	return StartCluster(t, cluster).URL
}

func StartCluster(t *testing.T, cluster Cluster) *FakeCluster {
	t.Helper()
	fake := &FakeCluster{cluster: cluster, refused: map[string]int32{}, unchanged: map[string]int32{}}
	fake.Add(t, cluster.Objects...)
	fake.URL = serve(t, fake)
	return fake
}

func (f *FakeCluster) Add(t *testing.T, objects ...runtime.Object) {
	t.Helper()
	f.mutex.Lock()
	defer f.mutex.Unlock()
	for _, object := range objects {
		stored := store(t, object)
		f.keep(stored, f.index(apiPath{group: stored.group, resource: stored.resource, namespace: stored.namespace, name: stored.name}), false)
	}
}

func (f *FakeCluster) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := route(r.URL.Path)
	switch status, refused := f.refusal(r, path); {
	case slices.Contains(f.cluster.Forbidden, path.qualified()):
		writeJSON(w, http.StatusForbidden,
			failure(http.StatusForbidden, metav1.StatusReasonForbidden, fmt.Sprintf(forbiddenFormat, path.qualified())))
	case refused:
		writeJSON(w, int(status), failure(status, "", fmt.Sprintf(refusedFormat, path.request(r.Method))))
	case r.URL.Path == versionPath:
		writeJSON(w, http.StatusOK, version.Info{GitVersion: cmp.Or(f.cluster.Version, DefaultVersion)})
	case discovers(r.URL.Path, path):
		f.discover(w, r.URL.Path, path)
	case r.Method == http.MethodGet:
		f.read(w, r, path)
	case path.resource == reviewsResource:
		f.review(w, r)
	case path.resource == whoAmIResource:
		f.whoAmI(w)
	default:
		f.write(w, r, path)
	}
}

// Refuse gives one request its status from now on. The request reads like "PATCH configmaps/telark-settings", and
// its name stays empty for a list or a create: "POST secrets/".
func (f *FakeCluster) Refuse(request string, status int32) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.refused[request] = status
}

// RefuseChange refuses the request only when it would change something: its dry run still passes.
func (f *FakeCluster) RefuseChange(request string, status int32) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.unchanged[request] = status
}

func (f *FakeCluster) refusal(r *http.Request, path apiPath) (int32, bool) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	status, refused := f.refused[path.request(r.Method)]
	if !refused && !r.URL.Query().Has(dryRunParam) {
		status, refused = f.unchanged[path.request(r.Method)]
	}
	return status, refused
}

func (p apiPath) request(method string) string {
	return fmt.Sprintf(requestFormat, method, p.qualified(), p.name)
}

func (f *FakeCluster) read(w http.ResponseWriter, r *http.Request, path apiPath) {
	if r.URL.Query().Has(watchParam) {
		f.watch(w, r, path)
		return
	}
	f.mutex.Lock()
	defer f.mutex.Unlock()
	if path.name != "" {
		f.get(w, path)
		return
	}
	f.list(w, r, path)
}

func (f *FakeCluster) get(w http.ResponseWriter, path apiPath) {
	for _, object := range f.objects {
		if object.matches(path) && object.name == path.name {
			writeJSON(w, http.StatusOK, object.content)
			return
		}
	}
	writeJSON(w, http.StatusNotFound, missing(path))
}

func missing(path apiPath) metav1.Status {
	return failure(http.StatusNotFound, metav1.StatusReasonNotFound, fmt.Sprintf(notFoundFormat, path.qualified(), path.name))
}

func (f *FakeCluster) list(w http.ResponseWriter, r *http.Request, path apiPath) {
	selector, err := labels.Parse(r.URL.Query().Get(selectorParam))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, failure(http.StatusBadRequest, metav1.StatusReasonBadRequest, err.Error()))
		return
	}
	named := fields.ParseSelectorOrDie(r.URL.Query().Get(fieldParam))
	items := []map[string]any{}
	for _, object := range f.objects {
		if object.matches(path) && selector.Matches(labels.Set(object.labels)) && named.Matches(object.fields()) {
			items = append(items, object.content)
		}
	}
	list := unstructured.UnstructuredList{Object: map[string]any{"metadata": map[string]any{}}}
	list.SetAPIVersion(path.groupVersion())
	list.SetKind(kindOf(path) + listSuffix)
	content := list.UnstructuredContent()
	content["items"] = items
	writeJSON(w, http.StatusOK, content)
}

func (f *FakeCluster) review(w http.ResponseWriter, r *http.Request) {
	// client-go sends built-in kinds as protobuf, which the universal deserializer reads as well as JSON.
	var review authorizationv1.SelfSubjectAccessReview
	body, err := io.ReadAll(r.Body)
	if err == nil {
		_, _, err = scheme.Codecs.UniversalDeserializer().Decode(body, nil, &review)
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, failure(http.StatusBadRequest, metav1.StatusReasonBadRequest, err.Error()))
		return
	}
	attributes := review.Spec.ResourceAttributes
	review.Status.Allowed = !slices.Contains(f.cluster.Denied, apiPath{group: attributes.Group, resource: attributes.Resource}.qualified())
	writeJSON(w, http.StatusCreated, review)
}

func (f *FakeCluster) whoAmI(w http.ResponseWriter) {
	review := authenticationv1.SelfSubjectReview{TypeMeta: metav1.TypeMeta{APIVersion: whoAmIVersion, Kind: whoAmIKind}}
	review.Status.UserInfo.Username = cmp.Or(f.cluster.User, DefaultUser)
	writeJSON(w, http.StatusCreated, review)
}

// A namespaced path with no namespace lists across all of them, as the real API server does.
func (o storedObject) matches(path apiPath) bool {
	return o.group == path.group && o.resource == path.resource && (path.namespace == "" || o.namespace == path.namespace)
}

func route(urlPath string) apiPath {
	var path apiPath
	parts := strings.Split(strings.Trim(urlPath, pathSeparator), pathSeparator)
	switch {
	case len(parts) >= coreSkip && parts[first] == corePrefix:
		path.version, parts = parts[second], parts[coreSkip:]
	case len(parts) >= groupSkip && parts[first] == groupPrefix:
		path.group, path.version, parts = parts[second], parts[third], parts[groupSkip:]
	default:
		// /version and the other root paths name only their resource.
	}
	if len(parts) > coreSkip && parts[first] == namespacesPath {
		path.namespace, parts = parts[second], parts[coreSkip:]
	}
	if len(parts) > first {
		path.resource = parts[first]
	}
	if len(parts) > second {
		path.name = parts[second]
	}
	return path
}

func (p apiPath) qualified() string {
	if p.group == "" {
		return p.resource
	}
	return p.resource + groupSeparator + p.group
}

// Typed clients decode a list by its kind, so an empty list still needs the kind client-go's scheme knows, or the
// served kind of a custom resource.
func kindOf(path apiPath) string {
	for gvk := range scheme.Scheme.AllKnownTypes() {
		plural, _ := meta.UnsafeGuessKindToResource(gvk)
		if gvk.Group == path.group && gvk.Version == path.version && plural.Resource == path.resource {
			return gvk.Kind
		}
	}
	if slices.ContainsFunc(served, path.serves) {
		return servedKind(path)
	}
	return ""
}

func store(t *testing.T, object runtime.Object) storedObject {
	t.Helper()
	gvk := object.GetObjectKind().GroupVersionKind()
	if gvk.Empty() {
		kinds, _, err := scheme.Scheme.ObjectKinds(object)
		if err != nil {
			t.Fatal(err)
		}
		gvk = kinds[first]
	}
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(object)
	if err != nil {
		t.Fatal(err)
	}
	item := unstructured.Unstructured{Object: content}
	item.SetGroupVersionKind(gvk)
	plural, _ := meta.UnsafeGuessKindToResource(gvk)
	return storedObject{
		group: gvk.Group, resource: plural.Resource, namespace: item.GetNamespace(), name: item.GetName(),
		labels: item.GetLabels(), content: item.Object,
	}
}

func ForbiddenAPIServer(t *testing.T) string {
	t.Helper()
	return serve(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusForbidden, failure(http.StatusForbidden, metav1.StatusReasonForbidden, forbiddenMessage))
	}))
}

func ClosedServer(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.NotFoundHandler())
	server.Close()
	return server.URL
}

// Kubeconfig gives each server a cluster, user and context of the same name, and selects current.
func Kubeconfig(t *testing.T, current string, servers map[string]string) string {
	t.Helper()
	config := clientcmdapi.NewConfig()
	for name, server := range servers {
		config.Clusters[name] = &clientcmdapi.Cluster{Server: server}
		config.AuthInfos[name] = &clientcmdapi.AuthInfo{Token: testToken}
		config.Contexts[name] = &clientcmdapi.Context{Cluster: name, AuthInfo: name}
	}
	config.CurrentContext = current
	return WriteKubeconfig(t, config)
}

func WriteKubeconfig(t *testing.T, config *clientcmdapi.Config) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), kubeconfigName)
	if err := clientcmd.WriteToFile(*config, path); err != nil {
		t.Fatal(err)
	}
	return path
}

func serve(t *testing.T, handler http.Handler) string {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server.URL
}

func failure(code int32, reason metav1.StatusReason, message string) metav1.Status {
	return metav1.Status{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Status"},
		Status:   metav1.StatusFailure,
		Reason:   reason,
		Code:     code,
		Message:  message,
	}
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set(contentType, jsonType)
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
