package metrics

import (
	"testing"

	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/metrics/client"
)

func TestInitMetricsClient(t *testing.T) {
	client.ResetClient()
	_, err := client.InitMetricsClient()
	if err != nil {
		t.Logf(ExpectedInitMetricsClientReturnedError, err)
	}
}

func TestResetClient(t *testing.T) {
	client.ResetClient()
}

func TestNewMetricsAdapter(t *testing.T) {
	adapter, err := client.NewMetricsAdapter()
	if err != nil && adapter != nil {
		t.Errorf(ExpectedErrorForNilAdapter, adapter)
	}
}

func TestGetWorkloadMetrics_NilAdapter(t *testing.T) {
	_, err := client.GetWorkloadMetrics(nil, TestNamespace, TestSelectors)
	if err == nil || err.Error() != string(constants.ERROR_METRICS_ADAPTER_OR_CLIENT_NIL) {
		t.Errorf(ExpectedErrorForNilAdapterFormat, constants.ERROR_METRICS_ADAPTER_OR_CLIENT_NIL, err)
	}
}

func TestGetFirstPodMetrics_NilAdapter(t *testing.T) {
	_, err := client.GetFirstPodMetrics(nil, TestNamespace, TestSelectors)
	if err == nil || err.Error() != string(constants.ERROR_METRICS_ADAPTER_OR_CLIENT_NIL) {
		t.Errorf(ExpectedErrorForNilAdapterFormat, constants.ERROR_METRICS_ADAPTER_OR_CLIENT_NIL, err)
	}
}

func TestAdapterGetContainerMetrics_NilAdapter(t *testing.T) {
	_, err := client.AdapterGetContainerMetrics(nil, TestNamespace, TestPodName, TestContainerName)
	if err == nil || err.Error() != string(constants.ERROR_METRICS_ADAPTER_OR_CLIENT_NIL) {
		t.Errorf(ExpectedErrorForNilAdapterFormat, constants.ERROR_METRICS_ADAPTER_OR_CLIENT_NIL, err)
	}
}
