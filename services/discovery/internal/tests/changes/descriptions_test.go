package changes

import (
	"strings"
	"testing"

	appresource "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/core/applications/history/changes"
	"github.com/telark/discovery/internal/tests/testutil"
)

// Every exported description formatter renders a non-empty, trimmed string that
// embeds the values it was given.
func TestDescFormatters(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"image updated", changes.DescImageUpdated("a:1", "a:2"), "a:2"},
		{"image added", changes.DescImageAdded("a:1"), "a:1"},
		{"image removed", changes.DescImageRemoved("a:1"), "a:1"},
		{"resource added", changes.DescResourceAdded("Deployment", "web"), "web"},
		{"resource removed", changes.DescResourceRemoved("Deployment", "web"), "web"},
		{"port added", changes.DescPortAdded("8080"), "8080"},
		{"port removed", changes.DescPortRemoved("8080"), "8080"},
		{"env added", changes.DescEnvVarKeyAdded("LOG"), "LOG"},
		{"env removed", changes.DescEnvVarKeyRemoved("LOG"), "LOG"},
		{"configmap added", changes.DescConfigMapRefAdded("cm"), "cm"},
		{"configmap removed", changes.DescConfigMapRefRemoved("cm"), "cm"},
		{"secret added", changes.DescSecretRefAdded("sec"), "sec"},
		{"secret removed", changes.DescSecretRefRemoved("sec"), "sec"},
		{"svc mapping added", changes.DescServiceMappingAdded("m"), "m"},
		{"svc mapping removed", changes.DescServiceMappingRemoved("m"), "m"},
		{"ingress added", changes.DescIngressRuleAdded("r"), "r"},
		{"ingress removed", changes.DescIngressRuleRemoved("r"), "r"},
		{"chart version", changes.DescChartVersionUpdated("1.0", "2.0"), "2.0"},
		{"resource count", changes.DescResourceCountChanged("3", "5"), "5"},
		{"requests cpu", changes.DescRequestsCPUChanged("100m", "200m"), "200m"},
		{"requests memory", changes.DescRequestsMemoryChanged("1Gi", "2Gi"), "2Gi"},
		{"limits cpu", changes.DescLimitsCPUChanged("1", "2"), "2"},
		{"limits memory", changes.DescLimitsMemoryChanged("1Gi", "2Gi"), "2Gi"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if strings.TrimSpace(c.got) == "" {
				t.Fatalf("%s produced an empty description", c.name)
			}
			if !strings.Contains(c.got, c.want) {
				t.Fatalf("%s = %q, want it to contain %q", c.name, c.got, c.want)
			}
		})
	}
}

// An unknown field with no explicit description falls back to a "field: type"
// synthesis rather than an empty string.
func TestApplicationChangeDescriptionFallback(t *testing.T) {
	ch := appresource.ApplicationChange{Field: "somethingNovel", ChangeType: changes.ChangeTypeUpdated}
	got := changes.ApplicationChangeDescription(ch)
	if !strings.Contains(got, "somethingNovel") || !strings.Contains(got, changes.ChangeTypeUpdated) {
		t.Fatalf("fallback description = %q", got)
	}
}

// Removed change types synthesise from the old value across the config-style
// fields, exercising the removed branches of every synth helper.
func TestApplicationChangeDescriptionRemovedBranches(t *testing.T) {
	fields := []string{
		changes.ChangeFieldImage, changes.ChangeFieldResource, changes.ChangeFieldPort,
		changes.ChangeFieldEnvVarKey, changes.ChangeFieldConfigMapRef, changes.ChangeFieldSecretRef,
		changes.ChangeFieldServiceMapping, changes.ChangeFieldIngressRule,
	}
	for _, f := range fields {
		t.Run(f, func(t *testing.T) {
			ch := appresource.ApplicationChange{Field: f, ChangeType: changes.ChangeTypeRemoved, OldValue: strptr("old")}
			if got := changes.ApplicationChangeDescription(ch); got == "" {
				t.Fatalf("removed %s synthesised empty", f)
			}
		})
	}
}

// Metrics-baseline fields synthesise a transition string from old/new values.
func TestApplicationChangeDescriptionMetricsBaseline(t *testing.T) {
	fields := []string{
		changes.ChangeFieldRequestsCPU, changes.ChangeFieldRequestsMemory,
		changes.ChangeFieldLimitsCPU, changes.ChangeFieldLimitsMemory,
	}
	for _, f := range fields {
		t.Run(f, func(t *testing.T) {
			ch := appresource.ApplicationChange{Field: f, ChangeType: changes.ChangeTypeUpdated, OldValue: strptr("1"), NewValue: strptr("2")}
			got := changes.ApplicationChangeDescription(ch)
			testutil.Equal(t, "contains new", strings.Contains(got, "2"), true)
		})
	}
}
