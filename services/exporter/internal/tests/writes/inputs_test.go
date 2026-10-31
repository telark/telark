package writes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	categorydata "github.com/telark/telark/internal/data/classification/category"
	"github.com/telark/telark/internal/data/metadata/base"
	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	roledata "github.com/telark/telark/internal/data/resources/role"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
	"github.com/telark/telark/internal/kcore/k8sclient"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/exporters/generics"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

const (
	categoriesPath   = "/api/v1/categories"
	passkeysPath     = "/api/v1/internal/auth/passkeys"
	configPath       = "/api/v1/config"
	sessionsPath     = "/api/v1/internal/auth/users/" + userID + "/sessions"
	keyFullnameField = "fullname"
	keyExcluded      = "excludedNamespaces"
	keyUsername      = "username"
	keyUserAgent     = "userAgent"
	keyDeviceMeta    = "deviceMetadata"
	padded           = "  "
	overChar         = "a"
	trimmedName      = "release_lead"
	trimmedFullname  = "Release Lead"
	trimmedNamespace = "team-a"
	trimmedPlan      = "freeze"
	trimmedApp       = "Shop"
	laptop           = "laptop"
	newPasskey       = `{"credentialId":"c1","publicKey":"p1","deviceType":"platform","deviceName":"`
	closeObject      = `}`
	customCategoryID = "cat-0000a-0000-0001"
)

func over(limit int) string {
	return strings.Repeat(overChar, limit+constants.DefaultIncrementValue)
}

func pad(value string) string {
	return padded + value + padded
}

// A one-field JSON object with a string value.
func fieldBody(key, value string) string {
	return `{"` + key + `":"` + value + `"}`
}

func planSeed() seed {
	md := v1alpha1.ProtectionPlanMetadata
	return crSeed(md, object(md, planID, map[string]any{keyName: planID}, nil))
}

func categorySeed() seed {
	md := v1alpha1.CategoryMetadata
	category := map[string]any{
		keyID: customCategoryID, keyName: trimmedName, keyScope: roledata.ScopeApplications, keyType: string(categorydata.CategoryTypeCustom),
	}
	return crSeed(md, object(md, constants.CategoriesCRDName, map[string]any{constants.CategoriesCRDName: []any{category}}, nil))
}

func appSeed() seed {
	md := v1alpha1.ApplicationMetadata
	return crSeed(md, object(md, appName, map[string]any{keyName: appName}, nil))
}

// Each user-typed field past its limit answers 400 naming the field, before anything is written.
func TestOverLimitFieldsAnswer400BeforeAnyWrite(t *testing.T) {
	participants := `"` + strings.Repeat(userID+`","`, constants.MaxPlanParticipants) + userID + `"`
	cases := []struct {
		name, method, path, body, field string
		seeds                           func(t *testing.T) []seed
	}{
		{"user create", http.MethodPost, usersPath, fieldBody(keyFullnameField, over(constants.MaxFullnameLength)), keyFullnameField, nil},
		{"user patch", http.MethodPatch, usersPrefix + plainUser, fieldBody(keyUsername, "release lead"), keyUsername,
			func(t *testing.T) []seed { return []seed{adminUser(t, plainUser, nil, nil, false)} }},
		{"group create", http.MethodPost, groupsPath, fieldBody(keyName, "release leads"), keyName, nil},
		{"role create", http.MethodPost, rolesPath, fieldBody(keyDesc, over(constants.MaxRoleDescriptionLength)), keyDesc, nil},
		{"category create", http.MethodPost, categoriesPath, fieldBody(keyName, over(constants.MaxCategoryNameLength)), keyName, nil},
		{"category patch", http.MethodPatch, categoriesPath + pathSep + customCategoryID, fieldBody(keyName, padded), keyName,
			func(*testing.T) []seed { return []seed{categorySeed()} }},
		{"plan create", http.MethodPost, plansPath, `{"participantRefs":[` + participants + `]}`, constants.FieldParticipantRefs, nil},
		{"plan patch", http.MethodPatch, plansPath + pathSep + planID, fieldBody(keyName, padded), keyName,
			func(*testing.T) []seed { return []seed{planSeed()} }},
		{"application patch", http.MethodPatch, appPath, fieldBody(keyDisplayName, over(constants.MaxApplicationDisplayNameLength)),
			keyDisplayName, func(*testing.T) []seed { return []seed{appSeed()} }},
		{"config patch", http.MethodPatch, configPath, `{"excludedNamespaces":["Team_A"]}`, keyExcluded, nil},
		{"config key set", http.MethodPatch, configPath,
			`{"oidc":` + fieldBody(telarkconfig.OIDCSecretKey, over(constants.MaxOIDCJWKBytes)) + closeObject, telarkconfig.OIDCSecretKey, nil},
		{"passkey create", http.MethodPost, passkeysPath, newPasskey + over(constants.MaxPasskeyDeviceNameLength) + `"}`, keyDeviceName,
			func(t *testing.T) []seed { return []seed{adminUser(t, userID, nil, nil, false)} }},
		{"role validity", http.MethodPost, rolesPath, `{"validity":{"type":"temporary","durationHours":8761}}`, constants.FieldValidityHours, nil},
		{"role patch validity", http.MethodPatch, rolesPath + pathSep + storedRoleID, `{"validity":{"type":"temporary","durationHours":0}}`,
			constants.FieldValidityHours, func(t *testing.T) []seed { return []seed{storedRole(t, roledata.Protection{})} }},
		{"config snapshots", http.MethodPatch, configPath, `{"snapshots":{"maxPerApp":101}}`, constants.FieldSnapshotsMaxPerApp, nil},
		{"passkey patch", http.MethodPatch, passkeysPath + pathSep + ownPasskey, fieldBody(keyDeviceName, padded), keyDeviceName,
			func(*testing.T) []seed { return []seed{passkeySeed(ownPasskey, userID)} }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var seeds []seed
			if c.seeds != nil {
				seeds = c.seeds(t)
			}
			client := installFake(t, seeds...)
			w := serviceCall(t, c.method, c.path, c.body)
			if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), c.field) {
				t.Fatalf("%s = %d %s, want 400 naming %s", c.name, w.Code, w.Body.String(), c.field)
			}
			if got := writes(t, client); len(got) != constants.DefaultInitValue {
				t.Fatalf("writes = %+v, want none", got)
			}
		})
	}
}

func onlyObject(t *testing.T, client *dynamicfake.FakeDynamicClient, md base.Metadata) map[string]any {
	t.Helper()
	list, err := client.Resource(gvrOf(md)).Namespace(md.Namespace).List(t.Context(), metav1.ListOptions{})
	if err != nil || len(list.Items) != constants.DefaultIncrementValue {
		t.Fatalf("stored %s = %v (err %v), want one", md.Plural, list, err)
	}
	return list.Items[firstItem].Object
}

// What reaches the cluster is the trimmed value, and browser metadata or a failure reason past its limit is cut, not refused.
func TestWritesStoreTrimmedValues(t *testing.T) {
	paddedPlan := strings.Replace(newPlan, `"`+trimmedPlan+`"`, `"`+pad(trimmedPlan)+`"`, constants.DefaultIncrementValue)
	newUser := `{"username":"` + pad(trimmedName) + `","fullname":"` + pad(trimmedFullname) + `","email":"test@example.com"}`
	cases := []struct {
		name, method, path, body string
		seeds                    []seed
		md                       base.Metadata
		keys                     []string
		want                     any
	}{
		{"user create", http.MethodPost, usersPath, newUser, nil, v1alpha1.UserMetadata, []string{keySpec, keyFullnameField}, trimmedFullname},
		{"user create username", http.MethodPost, usersPath, newUser, nil, v1alpha1.UserMetadata, []string{keySpec, keyUsername}, trimmedName},
		{"plan create", http.MethodPost, plansPath, paddedPlan, nil, v1alpha1.ProtectionPlanMetadata, []string{keySpec, keyName}, trimmedPlan},
		{"application patch", http.MethodPatch, appPath, `{"spec":` + fieldBody(keyDisplayName, pad(trimmedApp)) + closeObject,
			[]seed{appSeed()}, v1alpha1.ApplicationMetadata, []string{keySpec, keyDisplayName}, trimmedApp},
		{"passkey patch", http.MethodPatch, passkeysPath + pathSep + ownPasskey, fieldBody(keyDeviceName, pad(laptop)),
			[]seed{passkeySeed(ownPasskey, userID)}, v1alpha1.PasskeyMetadata, []string{keySpec, keyDeviceName}, laptop},
		{"config patch", http.MethodPatch, configPath, `{"excludedNamespaces":["` + pad(trimmedNamespace) + `"]}`,
			[]seed{defaultConfig()}, v1alpha1.TelarkConfigMetadata, []string{keySpec, keyExcluded}, []any{trimmedNamespace}},
		{"plan failure reason", http.MethodPatch, plansPath + pathSep + planID,
			fieldBody(constants.FieldPlanReason, over(constants.MaxPlanReasonLength)), []seed{planSeed()},
			v1alpha1.ProtectionPlanMetadata, []string{keyStatus, constants.FieldPlanReason},
			strings.Repeat(overChar, constants.MaxPlanReasonLength)},
		{"session create", http.MethodPost, sessionsPath,
			`{"sessionToken":"` + forgedID + `","deviceMetadata":` + fieldBody(keyUserAgent, over(constants.MaxSessionMetadataLength)) + closeObject,
			nil, v1alpha1.SessionMetadata, []string{keySpec, keyDeviceMeta, keyUserAgent},
			strings.Repeat(overChar, constants.MaxSessionMetadataLength)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client := installFake(t, c.seeds...)
			k8sclient.SetDynamicClient(jsonCreates{client})
			if w := serviceCall(t, c.method, c.path, c.body); w.Code != http.StatusOK {
				t.Fatalf("%s = %d %s", c.name, w.Code, w.Body.String())
			}
			got, _, _ := unstructured.NestedFieldNoCopy(onlyObject(t, client, c.md), c.keys...)
			if !equalJSON(t, got, c.want) {
				t.Fatalf("stored %v = %#v, want %#v", c.keys, got, c.want)
			}
		})
	}
}

func equalJSON(t *testing.T, got, want any) bool {
	t.Helper()
	return string(mustJSON(t, got)) == string(mustJSON(t, want))
}

// A patch the CRD schema refuses is the caller's mistake: 422 like a create, not 500.
func TestSchemaRefusedPatchAnswers422(t *testing.T) {
	md := v1alpha1.ApplicationMetadata
	client := installFake(t, appSeed())
	client.PrependReactor(verbPatch, md.Plural, func(k8stesting.Action) (bool, runtime.Object, error) {
		cause := field.Invalid(field.NewPath(keySpec, keyDisplayName), newName, overChar)
		return true, nil, k8serrors.NewInvalid(schema.GroupKind{Group: md.BaseGroup, Kind: md.Kind}, appName, field.ErrorList{cause})
	})
	w := httptest.NewRecorder()
	generics.GenericPatchCustomResource(w, md, appName, map[string]any{keySpec: map[string]any{keyDisplayName: newName}})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("patch = %d %s, want 422", w.Code, w.Body.String())
	}
}
