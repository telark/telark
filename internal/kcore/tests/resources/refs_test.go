package resources

import (
	"slices"
	"testing"

	"github.com/telark/kcore/resources/group"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestWorkloadConfigRefs(t *testing.T) {
	dep := &unstructured.Unstructured{Object: map[string]any{
		FieldKind: "Deployment",
		FieldSpec: map[string]any{FieldTemplate: map[string]any{FieldSpec: map[string]any{
			FieldContainers: []any{map[string]any{
				FieldEnvFrom: []any{
					map[string]any{FieldConfigMapRef: map[string]any{FieldName: "cfg"}},
					map[string]any{FieldSecretRef: map[string]any{FieldName: "creds"}},
				},
				FieldEnv: []any{map[string]any{FieldValueFrom: map[string]any{
					FieldSecretKeyRef: map[string]any{FieldName: "token", FieldKey: "t"},
				}}},
			}},
			FieldVolumes: []any{
				map[string]any{FieldConfigMap: map[string]any{FieldName: "cfg"}},
				map[string]any{FieldSecret: map[string]any{FieldSecretName: "tls"}},
			},
		}}},
	}}
	cms, secs := group.WorkloadConfigRefs(dep)
	if !slices.Equal(cms, []string{"cfg"}) {
		t.Fatalf(ExpectedConfigMapRefs, cms)
	}
	if !slices.Equal(secs, []string{"creds", "tls", "token"}) {
		t.Fatalf(ExpectedSecretRefs, secs)
	}
	cron := &unstructured.Unstructured{Object: map[string]any{
		FieldKind: "CronJob",
		FieldSpec: map[string]any{FieldJobTemplate: map[string]any{FieldSpec: map[string]any{
			FieldTemplate: map[string]any{FieldSpec: map[string]any{
				FieldContainers: []any{map[string]any{FieldEnvFrom: []any{
					map[string]any{FieldConfigMapRef: map[string]any{FieldName: "cron-cfg"}},
				}}},
			}},
		}}},
	}}
	cms, _ = group.WorkloadConfigRefs(cron)
	if !slices.Equal(cms, []string{"cron-cfg"}) {
		t.Fatalf(ExpectedCronJobConfigMapRefs, cms)
	}
}
