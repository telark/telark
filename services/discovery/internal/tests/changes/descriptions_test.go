package changes

import (
	"strings"
	"testing"

	appresource "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/core/applications/history/changes"
	"github.com/telark/discovery/internal/tests/testutil"
)

const (
	imageA1        = "a:1"
	resourceWeb    = "web"
	port8080       = "8080"
	envKeyLog      = "LOG"
	configMapRef   = "cm"
	secretRef      = "sec"
	serviceMapping = "m"
	ingressRule    = "r"
	memory2Gi      = "2Gi"
)

// Every exported description formatter renders a non-empty, trimmed string that
// embeds the values it was given.
func TestDescFormatters(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"image updated", changes.DescImageUpdated(imageA1, "a:2"), "a:2"},
		{"image added", changes.DescImageAdded(imageA1), imageA1},
		{"image removed", changes.DescImageRemoved(imageA1), imageA1},
		{"resource added", changes.DescResourceAdded("Deployment", resourceWeb), resourceWeb},
		{"resource removed", changes.DescResourceRemoved("Deployment", resourceWeb), resourceWeb},
		{"port added", changes.DescPortAdded(port8080), port8080},
		{"port removed", changes.DescPortRemoved(port8080), port8080},
		{"env added", changes.DescEnvVarKeyAdded(envKeyLog), envKeyLog},
		{"env removed", changes.DescEnvVarKeyRemoved(envKeyLog), envKeyLog},
		{"configmap added", changes.DescConfigMapRefAdded(configMapRef), configMapRef},
		{"configmap removed", changes.DescConfigMapRefRemoved(configMapRef), configMapRef},
		{"secret added", changes.DescSecretRefAdded(secretRef), secretRef},
		{"secret removed", changes.DescSecretRefRemoved(secretRef), secretRef},
		{"svc mapping added", changes.DescServiceMappingAdded(serviceMapping), serviceMapping},
		{"svc mapping removed", changes.DescServiceMappingRemoved(serviceMapping), serviceMapping},
		{"ingress added", changes.DescIngressRuleAdded(ingressRule), ingressRule},
		{"ingress removed", changes.DescIngressRuleRemoved(ingressRule), ingressRule},
		{"chart version", changes.DescChartVersionUpdated("1.0", "2.0"), "2.0"},
		{"resource count", changes.DescResourceCountChanged("3", "5"), "5"},
		{"requests cpu", changes.DescRequestsCPUChanged("100m", "200m"), "200m"},
		{"requests memory", changes.DescRequestsMemoryChanged("1Gi", memory2Gi), memory2Gi},
		{"limits cpu", changes.DescLimitsCPUChanged("1", "2"), "2"},
		{"limits memory", changes.DescLimitsMemoryChanged("1Gi", memory2Gi), memory2Gi},
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

// Removed change types synthesize from the old value across the config-style
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
				t.Fatalf("removed %s synthesized empty", f)
			}
		})
	}
}

// Metrics-baseline fields synthesize a transition string from old/new values.
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
