package naming

import (
	"maps"
	"slices"
	"testing"

	"github.com/telark/data/metadata/base"
	"github.com/telark/data/metadata/v1alpha1"
	"github.com/telark/data/policies"
	"github.com/telark/data/resources/finalizers"
	"github.com/telark/data/resources/telarkconfig"
	globalshared "github.com/telark/data/shared"
)

const (
	wantGroup      = "telark.io"
	wantVersion    = "v1alpha1"
	wantAPIVersion = "telark.io/v1alpha1"
)

type metadataCase struct {
	name      string
	metadata  base.Metadata
	kind      string
	plural    string
	singleton string
	status    []string
}

func metadataCases() []metadataCase {
	return []metadataCase{
		{name: "application", metadata: v1alpha1.ApplicationMetadata, kind: "Application", plural: "applications",
			status: []string{"health", "resourceCount", "namespaces", "resourceSummary", "resources", "images", "ports",
				"envVarKeys", "configMapRefs", "secretRefs", "serviceMappings", "ingressRules", "metrics", "snapshots",
				"rollbacks", "history", "lastForceSync", "createdAt", "lastUpdated", "conditions"}},
		{name: "protection plan", metadata: v1alpha1.ProtectionPlanMetadata, kind: "ProtectionPlan", plural: "protectionplans",
			status: []string{"phase", "reason", "conditions", "observedGeneration", "renderedPolicies", "health",
				"healthCheckedAt", "healthDetail", "startedAt", "startedBy", "terminatedAt", "terminatedBy", "approval"}},
		{name: "telark config", metadata: v1alpha1.TelarkConfigMetadata, kind: "TelarkConfig", plural: "telarkconfigs",
			singleton: "default", status: []string{"cluster"}},
		{name: "category", metadata: v1alpha1.CategoryMetadata, kind: "Category", plural: "categories", singleton: "categories"},
		{name: "user", metadata: v1alpha1.UserMetadata, kind: "User", plural: "users"},
		{name: "group", metadata: v1alpha1.GroupMetadata, kind: "Group", plural: "groups"},
		{name: "access role", metadata: v1alpha1.AccessRoleMetadata, kind: "AccessRole", plural: "accessroles"},
		{name: "passkey", metadata: v1alpha1.PasskeyMetadata, kind: "Passkey", plural: "passkeys"},
		{name: "session", metadata: v1alpha1.SessionMetadata, kind: "Session", plural: "sessions"},
	}
}

func TestMetadataMatchesContract(t *testing.T) {
	for _, tc := range metadataCases() {
		t.Run(tc.name, func(t *testing.T) {
			md := tc.metadata
			if md.BaseGroup != wantGroup || md.Version != wantVersion || md.GetAPIVersion() != wantAPIVersion {
				t.Errorf("group/version = %s, %s, want %s", md.BaseGroup, md.Version, wantAPIVersion)
			}
			if md.Kind != tc.kind || md.Plural != tc.plural {
				t.Errorf("kind/plural = %s/%s, want %s/%s", md.Kind, md.Plural, tc.kind, tc.plural)
			}
			if md.Namespace != globalshared.BaseNamespace {
				t.Errorf("namespace = %s, want %s", md.Namespace, globalshared.BaseNamespace)
			}
			if md.Singleton != tc.singleton {
				t.Errorf("singleton = %q, want %q", md.Singleton, tc.singleton)
			}
			assertFlatStatus(t, md.StatusFields, tc.status)
		})
	}
}

func assertFlatStatus(t *testing.T, got map[string]string, want []string) {
	t.Helper()
	if !slices.Equal(slices.Sorted(maps.Keys(got)), slices.Sorted(slices.Values(want))) {
		t.Errorf("status fields = %v, want %v", slices.Sorted(maps.Keys(got)), want)
	}
	for view, status := range got {
		if view != status {
			t.Errorf("status field %q projects %q, want the same key", view, status)
		}
	}
}

func TestPolicyLabelsAndAnnotations(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{name: "plan id", got: policies.LabelPlanID, want: "telark.io/protection-plan"},
		{name: "template", got: policies.LabelTemplate, want: "telark.io/template-id"},
		{name: "managed by", got: policies.LabelManagedBy, want: "app.kubernetes.io/managed-by"},
		{name: "managed by value", got: policies.ManagedByValue, want: "telark"},
		{name: "plan name", got: policies.AnnotationPlanName, want: "telark.io/plan-name"},
		{name: "created by", got: policies.AnnotationCreatedBy, want: "telark.io/created-by"},
		{name: "render hash", got: policies.AnnotationRenderHash, want: "telark.io/render-hash"},
		{name: "roles cleanup type", got: finalizers.ResourceTypeRoles, want: "accessroles"},
		{name: "config object", got: telarkconfig.TelarkConfigResourceName, want: "default"},
		{name: "oidc secret key", got: telarkconfig.OIDCSecretKey, want: "googleJwkJson"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("got %q, want %q", tc.got, tc.want)
			}
		})
	}
}
