package writes

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/telark/telark/internal/data/metadata/base"
	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	roledata "github.com/telark/telark/internal/data/resources/role"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
	userdata "github.com/telark/telark/internal/data/resources/user"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/exporters/generics"
	confighandler "github.com/telark/telark/services/exporter/internal/handlers/config"
	"github.com/telark/telark/services/exporter/internal/startup"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

const (
	jwk      = `{"keys":[{"kid":"k1"}]}`
	oldJWK   = `{"keys":[]}`
	patchFmt = "%s: patch status = %d, body %s"

	appPath    = "/api/v1/applications/" + appName
	keyLabels  = "labels"
	rollbackID = "rb-1"

	crdDir        = "../../../../../charts/telark-crds/templates/crds"
	crdUsers      = "users.yaml"
	crdConfigs    = "telarkconfigs.yaml"
	keyVersions   = "versions"
	keySchema     = "schema"
	keyOpenAPI    = "openAPIV3Schema"
	keyProperties = "properties"
	templateOpen  = "{{"
	lineBreak     = "\n"
	pathDot       = "."
)

func mustView(t *testing.T, obj *unstructured.Unstructured) map[string]any {
	t.Helper()
	out, err := sharedutils.FilterData(obj)
	if err != nil {
		t.Fatal(err)
	}
	single, ok := out.(*unstructured.Unstructured)
	if !ok {
		t.Fatalf("FilterData returned %T", out)
	}
	return single.Object
}

// The CRDs carry no spec.id: the REST view names a record by metadata.name.
func TestViewInjectsIDFromMetadataName(t *testing.T) {
	view := mustView(t, object(v1alpha1.UserMetadata, userID, map[string]any{keyName: newName}, nil))
	if view[keyID] != userID {
		t.Fatalf("view id = %v, want %q", view[keyID], userID)
	}

	list, err := sharedutils.FilterData(&unstructured.UnstructuredList{Items: []unstructured.Unstructured{
		*object(v1alpha1.UserMetadata, userID, map[string]any{keyID: forgedID}, nil),
	}})
	if err != nil {
		t.Fatal(err)
	}
	items := list.(*unstructured.UnstructuredList).Items
	if items[firstItem].Object[keyID] != userID {
		t.Fatalf("list item id = %v, want metadata.name %q over a stored spec.id", items[firstItem].Object[keyID], userID)
	}
}

// Plans and applications project status flat; the config projects it under cluster.
func TestViewProjectsStatusPerKind(t *testing.T) {
	plan := mustView(t, object(v1alpha1.ProtectionPlanMetadata, planID,
		map[string]any{keyName: planID}, map[string]any{keyPhase: phaseActive}))
	if plan[keyPhase] != phaseActive {
		t.Errorf("plan phase = %v, want %q", plan[keyPhase], phaseActive)
	}

	cluster := map[string]any{keyVersion: "1.31"}
	config := mustView(t, object(v1alpha1.TelarkConfigMetadata, v1alpha1.TelarkConfigSingleton,
		map[string]any{}, map[string]any{keyCluster: cluster}))
	if got, ok := config[keyCluster].(map[string]any); !ok || got[keyVersion] != cluster[keyVersion] {
		t.Errorf("config cluster = %v, want %v", config[keyCluster], cluster)
	}

	user := mustView(t, object(v1alpha1.UserMetadata, userID,
		map[string]any{keyStatus: map[string]any{keyPhase: phaseActive}}, nil))
	if _, ok := user[keyStatus].(map[string]any); !ok {
		t.Errorf("user status (kept in spec) = %v, want the spec object", user[keyStatus])
	}
}

func patchApp(t *testing.T, spec map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	generics.GenericPatchCustomResource(w, v1alpha1.ApplicationMetadata, appName, map[string]any{constants.SpecField: spec})
	if w.Code != http.StatusOK {
		t.Fatalf(patchFmt, t.Name(), w.Code, w.Body.String())
	}
}

// A status key merged into the main resource is dropped by the API server
// without an error, so it must only ever travel to /status.
func TestPatchSendsStatusKeysOnlyToStatus(t *testing.T) {
	client := installFake(t, crSeed(v1alpha1.ApplicationMetadata,
		object(v1alpha1.ApplicationMetadata, appName, map[string]any{keyName: appName}, nil)))

	patchApp(t, map[string]any{keyDisplayName: newName, keyHealth: healthOK, keyID: forgedID})

	var main, status int
	for _, w := range writes(t, client) {
		switch w.subresource {
		case subStatus:
			status++
			if section(w, keyStatus)[keyHealth] != healthOK {
				t.Errorf(missingKeyFmt, t.Name(), keyHealth, subStatus, w.body)
			}
			if _, found := w.body[keySpec]; found {
				t.Errorf(unexpectedKeyFmt, t.Name(), keySpec, subStatus, w.body)
			}
		default:
			main++
			spec := section(w, keySpec)
			for _, key := range []string{keyHealth, keyID} {
				if _, found := spec[key]; found {
					t.Errorf(unexpectedKeyFmt, t.Name(), key, keySpec, w.body)
				}
			}
			if spec[keyDisplayName] != newName {
				t.Errorf(missingKeyFmt, t.Name(), keyDisplayName, keySpec, w.body)
			}
		}
	}
	if main != constants.DefaultIncrementValue || status != constants.DefaultIncrementValue {
		t.Fatalf("main patches = %d, status patches = %d, want one each", main, status)
	}

	stored := stored(t, client, v1alpha1.ApplicationMetadata, appName)
	if health, _, _ := unstructured.NestedString(stored.Object, keyStatus, keyHealth); health != healthOK {
		t.Errorf("stored status.health = %q, want %q", health, healthOK)
	}
}

func TestStatusOnlyPatchSkipsTheMainResource(t *testing.T) {
	client := installFake(t, crSeed(v1alpha1.ApplicationMetadata,
		object(v1alpha1.ApplicationMetadata, appName, map[string]any{keyName: appName}, nil)))

	patchApp(t, map[string]any{keyHealth: healthOK})

	for _, w := range writes(t, client) {
		if w.subresource != subStatus {
			t.Errorf("status-only patch also wrote the main resource: %v", w.body)
		}
	}
}

// Discovery patches rollbacks and lastForceSync beside spec, not inside it.
func TestTopLevelStatusKeysReachTheStatusSubresource(t *testing.T) {
	internal := xauthz.Identity{Internal: true}
	appOwner := xauthz.Identity{UserID: callerID, Grants: xauthz.Grants{
		Levels: map[string]roledata.PermissionLevel{roledata.ScopeApplications: roledata.PermissionLevelOwner},
	}}
	none, one := constants.DefaultInitValue, constants.DefaultIncrementValue
	cases := []struct {
		name         string
		caller       xauthz.Identity
		body         map[string]any
		code         int
		statusKey    string
		mainWrites   int
		statusWrites int
	}{
		{"rollbacks", internal, map[string]any{v1alpha1.StatusRollbacks: []any{map[string]any{keyID: rollbackID}}},
			http.StatusOK, v1alpha1.StatusRollbacks, none, one},
		{"lastForceSync", internal, map[string]any{v1alpha1.StatusLastForceSync: map[string]any{keyPhase: phaseActive}},
			http.StatusOK, v1alpha1.StatusLastForceSync, none, one},
		{"root only", internal, map[string]any{constants.MetadataField: map[string]any{keyLabels: map[string]any{keyName: newName}}},
			http.StatusOK, constants.EmptyString, one, none},
		{"forged by a session", appOwner, map[string]any{v1alpha1.StatusRollbacks: []any{}},
			http.StatusForbidden, constants.EmptyString, none, none},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := installFake(t, crSeed(v1alpha1.ApplicationMetadata,
				object(v1alpha1.ApplicationMetadata, appName, map[string]any{keyName: appName}, nil)))

			w := patchAs(t, tc.caller, appPath, jsonBody(t, tc.body))
			if w.Code != tc.code {
				t.Fatalf(patchFmt, tc.name, w.Code, w.Body.String())
			}

			var main, status int
			for _, wr := range writes(t, client) {
				if wr.subresource != subStatus {
					main++
					continue
				}
				status++
				if _, found := section(wr, keyStatus)[tc.statusKey]; !found {
					t.Errorf(missingKeyFmt, tc.name, tc.statusKey, subStatus, wr.body)
				}
			}
			if main != tc.mainWrites || status != tc.statusWrites {
				t.Fatalf("main patches = %d, status patches = %d, want %d and %d", main, status, tc.mainWrites, tc.statusWrites)
			}
			if tc.statusKey != constants.EmptyString {
				obj := stored(t, client, v1alpha1.ApplicationMetadata, appName)
				if _, found, _ := unstructured.NestedFieldNoCopy(obj.Object, keyStatus, tc.statusKey); !found {
					t.Errorf("stored status lacks %q", tc.statusKey)
				}
			}
		})
	}
}

// Create drops .status when the CRD has a status subresource, so the projected
// keys are written by a second call.
func TestCreateWritesStatusThroughTheSubresource(t *testing.T) {
	client := installFake(t)

	w := httptest.NewRecorder()
	generics.GenericCreateCustomResource(w, v1alpha1.ProtectionPlanMetadata, planID,
		map[string]any{keyID: planID, keyName: planID, keyPhase: phaseActive})
	if w.Code != http.StatusOK {
		t.Fatalf("create status = %d, body %s", w.Code, w.Body.String())
	}

	var created, statusPatched bool
	for _, wr := range writes(t, client) {
		switch {
		case wr.subresource == subStatus:
			statusPatched = section(wr, keyStatus)[keyPhase] == phaseActive
		case section(wr, keySpec) != nil:
			created = true
			for _, key := range []string{keyID, keyPhase} {
				if _, found := section(wr, keySpec)[key]; found {
					t.Errorf(unexpectedKeyFmt, t.Name(), key, keySpec, wr.body)
				}
			}
		default:
		}
	}
	if !created || !statusPatched {
		t.Fatalf("created = %v, status patched = %v, want both", created, statusPatched)
	}
	if !strings.Contains(w.Body.String(), `"id":"`+planID+`"`) {
		t.Errorf("create response lacks the view id: %s", w.Body.String())
	}
}

// id is metadata.name; a spec.id would be pruned by the schema, and trusting one
// would let a body rename the record it describes.
func TestNoIDInAnySpecWrite(t *testing.T) {
	for _, md := range []base.Metadata{
		v1alpha1.UserMetadata, v1alpha1.GroupMetadata, v1alpha1.AccessRoleMetadata,
		v1alpha1.ProtectionPlanMetadata, v1alpha1.ApplicationMetadata,
	} {
		t.Run(md.Kind, func(t *testing.T) {
			client := installFake(t)
			create := httptest.NewRecorder()
			generics.GenericCreateCustomResource(create, md, userID, map[string]any{keyID: forgedID, keyName: newName})
			patch := httptest.NewRecorder()
			generics.GenericPatchCustomResource(patch, md, userID,
				map[string]any{constants.SpecField: map[string]any{keyID: forgedID, keyName: newName}})
			if create.Code != http.StatusOK || patch.Code != http.StatusOK {
				t.Fatalf("create = %d, patch = %d", create.Code, patch.Code)
			}

			for _, w := range writes(t, client) {
				if _, found := section(w, keySpec)[keyID]; found {
					t.Errorf(unexpectedKeyFmt, md.Kind, keyID, keySpec, w.body)
				}
			}
		})
	}
}

// The default config's cluster block is status, so the seed must write it
// through the subresource; built-in roles are created by name with no spec.id.
func TestSeedCreatesTheDefaultConfigAndBuiltinRoles(t *testing.T) {
	client := installFake(t)

	startup.SeedBuiltins()

	var roles, configCreates, configStatus int
	for _, w := range writes(t, client) {
		if _, found := section(w, keySpec)[keyID]; found {
			t.Errorf(unexpectedKeyFmt, w.resource, keyID, keySpec, w.body)
		}
		switch {
		case w.resource == v1alpha1.PluralAccessRoles && w.subresource == constants.EmptyString:
			roles++
		case w.resource == v1alpha1.PluralTelarkConfigs && w.subresource == subStatus:
			configStatus++
			assertKey(t, w, keyStatus, keyCluster, true)
		case w.resource == v1alpha1.PluralTelarkConfigs:
			configCreates++
			if w.name != v1alpha1.TelarkConfigSingleton {
				t.Errorf("config created as %q, want %q", w.name, v1alpha1.TelarkConfigSingleton)
			}
			assertKey(t, w, keySpec, keyCluster, false)
		default:
		}
	}
	if roles != len(roledata.BuiltinRoles) {
		t.Errorf("built-in roles created = %d, want %d", roles, len(roledata.BuiltinRoles))
	}
	if configCreates != constants.DefaultIncrementValue || configStatus != constants.DefaultIncrementValue {
		t.Errorf("config creates = %d, status writes = %d, want one each", configCreates, configStatus)
	}
}

func assertKey(t *testing.T, w write, sectionKey, key string, want bool) {
	t.Helper()
	if _, found := section(w, sectionKey)[key]; found != want {
		t.Errorf("%s: %s.%s present = %v, want %v: %v", w.resource, sectionKey, key, found, want, w.body)
	}
}

func trustSecret(value string) seed {
	return seed{gvr: secretGVR, obj: &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": secretVer,
		"kind":       secretKind,
		"metadata":   map[string]any{keyName: secretName, "namespace": v1alpha1.TelarkConfigMetadata.Namespace},
		keyData:      map[string]any{telarkconfig.OIDCSecretKey: base64.StdEncoding.EncodeToString([]byte(value))},
	}}}
}

func defaultConfig() seed {
	md := v1alpha1.TelarkConfigMetadata
	return crSeed(md, object(md, v1alpha1.TelarkConfigSingleton,
		map[string]any{keyOIDC: map[string]any{"enabled": false}}, nil))
}

func serveConfig(t *testing.T, handler http.HandlerFunc, method, body string) map[string]any {
	t.Helper()
	r := httptest.NewRequest(method, "/api/v1/config", strings.NewReader(body))
	r = r.WithContext(xauthz.WithIdentity(r.Context(), xauthz.Identity{Internal: true}))
	w := httptest.NewRecorder()
	handler(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("%s config = %d, body %s", method, w.Code, w.Body.String())
	}
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data
}

func viewJWK(view map[string]any) any {
	return section(write{body: view}, keyOIDC)[telarkconfig.OIDCSecretKey]
}

// The JWK goes to the trust Secret auth mounts, never into the CR, and the
// response still shows it where the UI reads it.
func TestConfigPatchWritesTheJWKToTheSecret(t *testing.T) {
	client := installFake(t, defaultConfig(), trustSecret(oldJWK))

	view := serveConfig(t, confighandler.PatchConfig(), http.MethodPatch,
		`{"oidc":{"enabled":true,"`+telarkconfig.OIDCSecretKey+`":`+jsonString(t, jwk)+`}}`)

	var secretWrites int
	for _, w := range writes(t, client) {
		if w.resource == secretPlural {
			secretWrites++
			encoded := section(w, keyData)[telarkconfig.OIDCSecretKey]
			if encoded != base64.StdEncoding.EncodeToString([]byte(jwk)) {
				t.Errorf("secret data = %v, want the encoded JWK", encoded)
			}
			continue
		}
		if strings.Contains(string(mustJSON(t, w.body)), telarkconfig.OIDCSecretKey) {
			t.Errorf("the JWK reached the CR: %v", w.body)
		}
	}
	if secretWrites != constants.DefaultIncrementValue {
		t.Fatalf("secret writes = %d, want 1", secretWrites)
	}
	if viewJWK(view) != jwk {
		t.Errorf("response JWK = %v, want %q", viewJWK(view), jwk)
	}
	if enabled, isBool := section(write{body: view}, keyOIDC)[keyEnabled].(bool); !isBool || !enabled {
		t.Errorf("the other oidc fields were not written: %v", view[keyOIDC])
	}
}

func TestConfigGetMergesTheJWK(t *testing.T) {
	installFake(t, defaultConfig(), trustSecret(jwk))

	view := serveConfig(t, confighandler.GetConfig(), http.MethodGet, constants.EmptyString)

	if viewJWK(view) != jwk {
		t.Errorf("GET config JWK = %v, want the Secret's %q", viewJWK(view), jwk)
	}
	if view[keyID] != v1alpha1.TelarkConfigSingleton {
		t.Errorf("config id = %v, want %q", view[keyID], v1alpha1.TelarkConfigSingleton)
	}
}

// The service writes the toggle through the same merge patch as every other setting,
// and GET shows it to settings readers beside the rest of the config.
func TestSelfRegistrationIsStoredAndShown(t *testing.T) {
	client := installFake(t, defaultConfig())

	serveConfig(t, confighandler.PatchConfig(), http.MethodPatch,
		`{"`+telarkconfig.FieldSelfRegistration+`":{"`+keyEnabled+`":true}}`)

	spec := storedSpec(t, client, v1alpha1.TelarkConfigMetadata, v1alpha1.TelarkConfigSingleton)
	enabled, found, err := unstructured.NestedBool(spec, telarkconfig.FieldSelfRegistration, keyEnabled)
	if err != nil || !found || !enabled {
		t.Fatalf("stored selfRegistration.enabled = %v (found %v, %v), want true", enabled, found, err)
	}
	if _, kept := spec[keyOIDC]; !kept {
		t.Errorf("the patch dropped the oidc block: %v", spec)
	}
	view := serveConfig(t, confighandler.GetConfig(), http.MethodGet, constants.EmptyString)
	if shown, isBool := section(write{body: view}, telarkconfig.FieldSelfRegistration)[keyEnabled].(bool); !isBool || !shown {
		t.Errorf("GET config selfRegistration = %v, want enabled", view[telarkconfig.FieldSelfRegistration])
	}
}

// The API server prunes what a schema does not declare and still answers 200, so every
// field written for these features must be in the CRD or it is silently dropped.
func TestCRDsDeclareTheNewFields(t *testing.T) {
	tests := []struct {
		file   string
		path   []string
		record any
	}{
		{crdUsers, []string{keySpec, keyStatus, constants.FieldInvite},
			userdata.Invite{IssuedAt: stampTime, ExpiresAt: stampTime, IssuedBy: callerID}},
		{crdUsers, []string{keySpec, keyStatus},
			userdata.UserStatus{Phase: phaseSuspended, InviteAcceptedAt: stampTime}},
		{crdConfigs, []string{keySpec, telarkconfig.FieldSelfRegistration},
			telarkconfig.SelfRegistrationConfig{Enabled: true}},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			declared := crdProperties(t, tt.file, tt.path)
			var fields map[string]any
			if err := json.Unmarshal(mustJSON(t, tt.record), &fields); err != nil {
				t.Fatal(err)
			}
			for field := range fields {
				if _, found := declared[field]; !found {
					t.Errorf("%s: %s.%s is not in the schema", tt.file, strings.Join(tt.path, pathDot), field)
				}
			}
		})
	}
}

// Template directives sit on lines of their own, and the schema is plain YAML without them.
func crdProperties(t *testing.T, file string, path []string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Clean(filepath.Join(crdDir, file)))
	if err != nil {
		t.Fatal(err)
	}
	lines := slices.DeleteFunc(strings.Split(string(raw), lineBreak), func(line string) bool {
		return strings.Contains(line, templateOpen)
	})
	var crd map[string]any
	if err := yaml.Unmarshal([]byte(strings.Join(lines, lineBreak)), &crd); err != nil {
		t.Fatal(err)
	}
	versions, _, err := unstructured.NestedSlice(crd, keySpec, keyVersions)
	if err != nil || len(versions) == firstItem {
		t.Fatalf("%s has no versions: %v", file, err)
	}
	version, isMap := versions[firstItem].(map[string]any)
	if !isMap {
		t.Fatalf("%s: version is %T", file, versions[firstItem])
	}
	properties, found, err := unstructured.NestedMap(version, keySchema, keyOpenAPI, keyProperties)
	for _, name := range path {
		if err != nil || !found {
			break
		}
		properties, found, err = unstructured.NestedMap(properties, name, keyProperties)
	}
	if err != nil || !found {
		t.Fatalf("%s: %s is not in the schema (%v)", file, strings.Join(path, pathDot), err)
	}
	return properties
}

func jsonString(t *testing.T, s string) string {
	t.Helper()
	return string(mustJSON(t, s))
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
