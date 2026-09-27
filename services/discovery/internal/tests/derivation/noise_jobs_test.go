package derivation

import (
	"testing"

	"github.com/telark/discovery/internal/discovery/derivation"
)

const (
	appLabelKey = "app"
	appA        = "a"
)

func TestCronJobOwnedJobsAreNoiseStandaloneJobsStay(t *testing.T) {
	in := []derivation.ResourceInput{
		{Namespace: inputNamespace, Kind: "CronJob", Name: "cron", Labels: map[string]string{appLabelKey: appA}},
		{
			Namespace: inputNamespace, Kind: "Job", Name: "cron-1", Labels: map[string]string{appLabelKey: appA},
			OwnerReferences: []derivation.OwnerReference{{Kind: "CronJob", Name: "cron"}},
		},
		{Namespace: inputNamespace, Kind: "Job", Name: "migrate", Labels: map[string]string{appLabelKey: appA}},
	}
	out := derivation.GroupByWorkloadAnchor(in)
	names := map[string]bool{}
	for _, r := range out {
		names[r.Kind+"/"+r.Name] = r.Group == appA
	}
	if names["Job/cron-1"] {
		t.Fatalf("CronJob-owned job must be filtered, got %v", names)
	}
	if !names["CronJob/cron"] || !names["Job/migrate"] {
		t.Fatalf("CronJob and standalone Job must group into app a, got %v", names)
	}
}
