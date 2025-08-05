package metrics

import (
	"testing"

	"github.com/plsyro/kcore/metrics"
)

func TestGetWorkloadQualityOfService_EmptyNamespace(t *testing.T) {
	qos := metrics.GetWorkloadQualityOfService(TestEmptyString, nil)
	if qos != TestEmptyString {
		t.Errorf(ExpectedEmptyStringForEmptyNamespace, qos)
	}
}

func TestBuildWorkloadUsage_EmptyNamespace(t *testing.T) {
	usage := metrics.BuildWorkloadUsage(TestEmptyString, nil, TestEmptyString)
	if usage != nil && usage.Available {
		t.Error(ExpectedUsageToBeUnavailableForEmptyNamespace)
	}
}
