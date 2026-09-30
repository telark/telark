package endpoints_test

import (
	"strings"
	"testing"

	"github.com/telark/telark/internal/rest/base"
	"github.com/telark/telark/internal/rest/endpoints/insights"
)

const (
	insightsPrefix = "insights/"
	tokenSegment   = "{token}"
)

func TestInsightsEndpointsHaveNoTokenSegment(t *testing.T) {
	for _, ep := range []base.Endpoint{
		insights.Applications,
		insights.Analyze,
		insights.Events,
		insights.Runtime,
		insights.RuntimeValidate,
		insights.RuntimePull,
	} {
		if !strings.HasPrefix(string(ep), insightsPrefix) {
			t.Errorf("%q does not start with %q", ep, insightsPrefix)
		}
		if strings.Contains(string(ep), tokenSegment) {
			t.Errorf("%q contains %q", ep, tokenSegment)
		}
	}
}
