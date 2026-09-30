package changes

import (
	"testing"

	appresource "github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/applications/history/changes"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	historyNamespace      = "prod"
	statusDown            = "down"
	portHTTP              = 80
	degradedTotalReplicas = 5
	minChangeCount        = 12
	portHTTPS             = 443
	statusHealthy         = "healthy"
)

func strptr(s string) *string { return &s }

// A stored/fresh pair that differs in every comparable field must produce a
// change for each field, and every emitted change must carry a rendered
// description, a known change type, and a field — this exercises the whole
// diff engine and the description builders behind it.
func TestCollectChangesEveryField(t *testing.T) {
	ver1, ver2 := "1.0.0", "2.0.0"
	stored := &appresource.Application{
		Name:            "shop",
		Images:          []string{"nginx:1.0"},
		Ports:           []int{portHTTP},
		EnvVarKeys:      []string{"A"},
		ConfigMapRefs:   []string{"cm1"},
		SecretRefs:      []string{"s1"},
		ServiceMappings: []string{"svc1"},
		IngressRules:    []string{"ing1"},
		Resources: []appresource.Resource{
			{Namespace: historyNamespace, Kind: "Deployment", Name: "shop-api"},
			{Namespace: historyNamespace, Kind: "Service", Name: "shop-svc"},
		},
		Health:  appresource.Health{Status: statusHealthy, ReadyReplicas: constants.ThreeValue, TotalReplicas: constants.ThreeValue},
		Managed: appresource.Managed{By: "helm", Version: &ver1},
	}
	fresh := &appresource.Application{
		Name:            "shop",
		Images:          []string{"nginx:2.0"},
		Ports:           []int{portHTTP, portHTTPS},
		EnvVarKeys:      []string{"A", "B"},
		ConfigMapRefs:   []string{"cm2"},
		SecretRefs:      []string{"s2"},
		ServiceMappings: []string{"svc2"},
		IngressRules:    []string{"ing2"},
		Resources: []appresource.Resource{
			{Namespace: historyNamespace, Kind: "Deployment", Name: "shop-api"},
			{Namespace: historyNamespace, Kind: "Service", Name: "shop-svc"},
			{Namespace: historyNamespace, Kind: "ConfigMap", Name: "shop-cfg"},
		},
		Health:  appresource.Health{Status: statusDown, ReadyReplicas: constants.DefaultInitValue, TotalReplicas: degradedTotalReplicas},
		Managed: appresource.Managed{By: "helm", Version: &ver2},
	}

	got := changes.CollectChanges(stored, fresh)
	if len(got) < minChangeCount {
		t.Fatalf("expected a change for each differing field, got %d", len(got))
	}
	known := map[string]bool{
		changes.ChangeTypeAdded: true, changes.ChangeTypeRemoved: true, changes.ChangeTypeUpdated: true,
	}
	for _, c := range got {
		if c.Field == "" {
			t.Errorf("change has empty field: %+v", c)
		}
		if c.Description == "" {
			t.Errorf("change %q has empty description", c.Field)
		}
		if !known[c.ChangeType] {
			t.Errorf("change %q has unknown type %q", c.Field, c.ChangeType)
		}
	}
}

// A change set that touches health-down classifies as an incident with high or
// critical severity, and the incident/recovery detectors agree.
func TestClassifySeverityIncidentRecovery(t *testing.T) {
	toDown := []appresource.ApplicationChange{{
		Field: changes.ChangeFieldHealth, ChangeType: changes.ChangeTypeUpdated,
		OldValue: strptr(statusHealthy), NewValue: strptr(statusDown),
	}}
	class := changes.ClassifyChanges(toDown)
	if class != appresource.ChangeClassIncident {
		t.Fatalf("health->down classified as %q, want incident", class)
	}
	if sev := changes.DetermineSeverity(class, toDown); sev != appresource.SeverityCritical {
		t.Fatalf("incident severity = %q, want critical", sev)
	}
	if !changes.DetectIncident(toDown, class) {
		t.Fatal("health->down not detected as incident")
	}

	toHealthy := []appresource.ApplicationChange{{
		Field: changes.ChangeFieldHealth, ChangeType: changes.ChangeTypeUpdated,
		OldValue: strptr(statusDown), NewValue: strptr(statusHealthy),
	}}
	if !changes.DetectRecovery(toHealthy) {
		t.Fatal("down->healthy not detected as recovery")
	}
	if changes.DetectIncident(toHealthy, changes.ClassifyChanges(toHealthy)) {
		t.Fatal("down->healthy wrongly detected as incident")
	}
}

// The fingerprint is empty for no changes, stable for the same set regardless of
// order, and differs when the set differs.
func TestComputeFingerprint(t *testing.T) {
	if fp := changes.ComputeFingerprint(nil); fp != constants.EmptyString {
		t.Fatalf("empty change set fingerprint = %q, want empty", fp)
	}
	a := []appresource.ApplicationChange{
		{Field: changes.ChangeFieldImage, ChangeType: changes.ChangeTypeAdded, NewValue: strptr("x")},
		{Field: changes.ChangeFieldPort, ChangeType: changes.ChangeTypeAdded, NewValue: strptr("80")},
	}
	reordered := []appresource.ApplicationChange{a[constants.DefaultAddValue], a[constants.DefaultInitValue]}
	if changes.ComputeFingerprint(a) != changes.ComputeFingerprint(reordered) {
		t.Fatal("fingerprint is order-sensitive")
	}
	b := []appresource.ApplicationChange{
		{Field: changes.ChangeFieldImage, ChangeType: changes.ChangeTypeAdded, NewValue: strptr("y")},
	}
	if changes.ComputeFingerprint(a) == changes.ComputeFingerprint(b) {
		t.Fatal("distinct change sets share a fingerprint")
	}
}

// With no explicit description, the description is synthesized from the field and
// change type; every field/type pair yields a non-empty string except a few
// no-op combinations, and the validator collapses whitespace and truncates.
func TestApplicationChangeDescriptionSynthesised(t *testing.T) {
	cases := []struct {
		name  string
		field string
		typ   string
	}{
		{"image added", changes.ChangeFieldImage, changes.ChangeTypeAdded},
		{"image removed", changes.ChangeFieldImage, changes.ChangeTypeRemoved},
		{"image updated", changes.ChangeFieldImage, changes.ChangeTypeUpdated},
		{"port added", changes.ChangeFieldPort, changes.ChangeTypeAdded},
		{"env added", changes.ChangeFieldEnvVarKey, changes.ChangeTypeAdded},
		{"resource added", changes.ChangeFieldResource, changes.ChangeTypeAdded},
		{"configmap added", changes.ChangeFieldConfigMapRef, changes.ChangeTypeAdded},
		{"secret removed", changes.ChangeFieldSecretRef, changes.ChangeTypeRemoved},
		{"service mapping added", changes.ChangeFieldServiceMapping, changes.ChangeTypeAdded},
		{"ingress added", changes.ChangeFieldIngressRule, changes.ChangeTypeAdded},
		{"requests cpu", changes.ChangeFieldRequestsCPU, changes.ChangeTypeUpdated},
		{"limits memory", changes.ChangeFieldLimitsMemory, changes.ChangeTypeUpdated},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ch := appresource.ApplicationChange{
				Field: c.field, ChangeType: c.typ,
				OldValue: strptr("old"), NewValue: strptr("new"),
			}
			if got := changes.ApplicationChangeDescription(ch); got == "" {
				t.Errorf("synthesized description is empty for %s/%s", c.field, c.typ)
			}
		})
	}
}

// An explicit description is preferred over synthesis, and the validator
// normalises internal whitespace.
func TestApplicationChangeDescriptionExplicit(t *testing.T) {
	ch := appresource.ApplicationChange{Field: changes.ChangeFieldImage, Description: "  a   b  "}
	testutil.Equal(t, "normalised", changes.ApplicationChangeDescription(ch), "a b")
	testutil.Equal(t, "blank", changes.ValidApplicationChangeDescription("   "), constants.EmptyString)
}
