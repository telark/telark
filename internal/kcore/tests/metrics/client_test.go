package metrics

import (
	"testing"

	"github.com/telark/kcore/constants"
	"github.com/telark/kcore/metrics/client"
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
	mc, err := client.InitMetricsClient()
	if mc == nil && err == nil {
		t.Error(ExpectedClientOrErrorAfterReset)
	}
}

func TestNewMetricsAdapter(t *testing.T) {
	adapter, err := client.NewMetricsAdapter()
	if err != nil && adapter != nil {
		t.Errorf(ExpectedErrorForNilAdapter, adapter)
	}
}

func TestGetWorkloadMetrics_NilAdapter(t *testing.T) {
	_, err := client.GetWorkloadMetrics(nil, TestNamespace, TestSelectors)
	if err == nil || err.Error() != string(constants.ErrMetricsAdapterOrClientNil) {
		t.Errorf(ExpectedErrorForNilAdapterFormat, constants.ErrMetricsAdapterOrClientNil, err)
	}
}

func TestGetAllPodMetrics_NilAdapter(t *testing.T) {
	_, err := client.GetAllPodMetrics(nil, TestNamespace, TestSelectors)
	if err == nil || err.Error() != string(constants.ErrMetricsAdapterOrClientNil) {
		t.Errorf(ExpectedErrorForNilAdapterFormat, constants.ErrMetricsAdapterOrClientNil, err)
	}
}

func TestAdapterGetContainerMetrics_NilAdapter(t *testing.T) {
	_, err := client.AdapterGetContainerMetrics(nil, TestNamespace, TestPodName, TestContainerName)
	if err == nil || err.Error() != string(constants.ErrMetricsAdapterOrClientNil) {
		t.Errorf(ExpectedErrorForNilAdapterFormat, constants.ErrMetricsAdapterOrClientNil, err)
	}
}
