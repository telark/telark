package derivation

import (
	"testing"

	"github.com/telark/discovery/internal/discovery/derivation"
)

func TestCronJobOwnedJobsAreNoiseStandaloneJobsStay(t *testing.T) {
	in := []derivation.ResourceInput{
		{Namespace: "n", Kind: "CronJob", Name: "cron", Labels: map[string]string{"app": "a"}},
		{Namespace: "n", Kind: "Job", Name: "cron-1", Labels: map[string]string{"app": "a"}, OwnerReferences: []derivation.OwnerReference{{Kind: "CronJob", Name: "cron"}}},
		{Namespace: "n", Kind: "Job", Name: "migrate", Labels: map[string]string{"app": "a"}},
	}
	out := derivation.GroupByWorkloadAnchor(in)
	names := map[string]bool{}
	for _, r := range out {
		names[r.Kind+"/"+r.Name] = r.Group == "a"
	}
	if names["Job/cron-1"] {
		t.Fatalf("CronJob-owned job must be filtered, got %v", names)
	}
	if !names["CronJob/cron"] || !names["Job/migrate"] {
		t.Fatalf("CronJob and standalone Job must group into app a, got %v", names)
	}
}
