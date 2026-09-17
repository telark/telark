package resources

import (
	"testing"

	"github.com/telark/kcore/resources/group"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestWorkloadConfigRefs(t *testing.T) {
	dep := &unstructured.Unstructured{Object: map[string]any{
		"kind": "Deployment",
		"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
			"containers": []any{map[string]any{
				"envFrom": []any{map[string]any{"configMapRef": map[string]any{"name": "cfg"}}, map[string]any{"secretRef": map[string]any{"name": "creds"}}},
				"env":     []any{map[string]any{"valueFrom": map[string]any{"secretKeyRef": map[string]any{"name": "token", "key": "t"}}}},
			}},
			"volumes": []any{map[string]any{"configMap": map[string]any{"name": "cfg"}}, map[string]any{"secret": map[string]any{"secretName": "tls"}}},
		}}},
	}}
	cms, secs := group.WorkloadConfigRefs(dep)
	if len(cms) != 1 || cms[0] != "cfg" {
		t.Fatalf("configmaps: %v", cms)
	}
	if len(secs) != 3 || secs[0] != "creds" || secs[1] != "tls" || secs[2] != "token" {
		t.Fatalf("secrets: %v", secs)
	}
	cron := &unstructured.Unstructured{Object: map[string]any{
		"kind": "CronJob",
		"spec": map[string]any{"jobTemplate": map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
			"containers": []any{map[string]any{"envFrom": []any{map[string]any{"configMapRef": map[string]any{"name": "cron-cfg"}}}}},
		}}}}},
	}}
	cms, _ = group.WorkloadConfigRefs(cron)
	if len(cms) != 1 || cms[0] != "cron-cfg" {
		t.Fatalf("cronjob configmaps: %v", cms)
	}
}
