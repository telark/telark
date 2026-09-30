package metrics

import "time"

const (
	TestNamespace         = "test-namespace"
	TestPodName           = "test-pod"
	TestContainerName     = "test-container"
	TestCPUString         = "500m"
	TestCPUStringTwoCores = "2"
	TestCPUValue          = 500
	TestCPUValueOneCore   = 1000
	TestCPUValueMultiCore = 2500
	TestCPUValueTwoCores  = 2000
	TestMemoryStringGi    = "1Gi"
	TestMemoryStringMi    = "512Mi"
	TestMemoryValueOneKi  = 1024
	TestMemoryValueOneMi  = 1048576
	TestMemoryValueGi     = 1073741824
	TestMemoryValueMi     = 536870912
	TestEmptyString       = ""
	TestInvalidCPU        = "notanumber"
	TestInvalidMemory     = "notanumber"
	TestSelectorKey       = "app"
	TestSelectorValue     = "test"
	TestRateLimitInterval = time.Nanosecond

	ExpectedFalseForNilMetricsClient              = "Expected false for nil MetricsClient"
	ExpectedFalseForNilMetricsAdapter             = "Expected false for nil MetricsAdapter"
	ExpectedFalseForUnreachableMetricsAPI         = "Expected false when the metrics API client is unset"
	ExpectedContainerMetricsStructFieldsNotSet    = "ContainerMetrics struct fields not set correctly"
	ExpectedPodMetricsStructFieldsNotSet          = "PodMetrics struct fields not set correctly"
	ExpectedMetricsClientStructNotCreated         = "MetricsClient struct not created"
	ExpectedMetricsAdapterStructNotCreated        = "MetricsAdapter struct not created"
	ExpectedErrorForNilAdapter                    = "Expected error for nil adapter %v"
	ExpectedErrorForNilAdapterFormat              = "Expected error '%s' for nil adapter, got: %v"
	ExpectedEmptyStringForEmptyNamespace          = "Expected empty string for empty namespace, got %q"
	ExpectedUsageToBeUnavailableForEmptyNamespace = "Expected usage to be unavailable for empty namespace"
	ExpectedFormatCPUReturnedEmptyString          = "FormatCPU(%d) returned empty string"
	ExpectedFormatMemoryReturnedEmptyString       = "FormatMemory(%d) returned empty string"
	ExpectedParseCPUReturned                      = "ParseCPU(%q) = %d, want %d"
	ExpectedParseMemoryReturned                   = "ParseMemory(%q) = %d, want %d"
	ExpectedInitMetricsClientReturnedError        = "InitMetricsClient returned error: %v"
	ExpectedClientOrErrorAfterReset               = "InitMetricsClient returned no client and no error after ResetClient"
	ExpectedConvertToPodMetrics                   = "ConvertToPodMetrics = %+v, want pod %q in namespace %q with no containers"
)

var TestSelectors = map[string]string{TestSelectorKey: TestSelectorValue}
