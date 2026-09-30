package metrics

import (
	"testing"

	"github.com/telark/kcore/constants"
	"github.com/telark/kcore/metrics/metricsutils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func TestFormatCPU(t *testing.T) {
	cases := []struct {
		input int64
	}{
		{TestCPUValue}, {TestCPUValueOneCore}, {TestCPUValueMultiCore},
	}
	for _, c := range cases {
		if got := metricsutils.FormatCPU(c.input); got == TestEmptyString {
			t.Errorf(ExpectedFormatCPUReturnedEmptyString, c.input)
		}
	}
}

func TestFormatMemory(t *testing.T) {
	cases := []struct {
		input int64
	}{
		{TestMemoryValueOneKi}, {TestMemoryValueOneMi}, {TestMemoryValueGi},
	}
	for _, c := range cases {
		if got := metricsutils.FormatMemory(c.input); got == TestEmptyString {
			t.Errorf(ExpectedFormatMemoryReturnedEmptyString, c.input)
		}
	}
}

func TestParseCPU(t *testing.T) {
	cases := []struct {
		input  string
		expect int64
	}{
		{TestCPUString, TestCPUValue},
		{TestCPUStringTwoCores, TestCPUValueTwoCores},
		{TestEmptyString, constants.ZeroValue},
		{TestInvalidCPU, constants.ZeroValue},
	}
	for _, c := range cases {
		if got := metricsutils.ParseCPU(c.input); got != c.expect {
			t.Errorf(ExpectedParseCPUReturned, c.input, got, c.expect)
		}
	}
}

func TestParseMemory(t *testing.T) {
	cases := []struct {
		input  string
		expect int64
	}{
		{TestMemoryStringGi, TestMemoryValueGi},
		{TestMemoryStringMi, TestMemoryValueMi},
		{TestEmptyString, constants.ZeroValue},
		{TestInvalidMemory, constants.ZeroValue},
	}
	for _, c := range cases {
		if got := metricsutils.ParseMemory(c.input); got != c.expect {
			t.Errorf(ExpectedParseMemoryReturned, c.input, got, c.expect)
		}
	}
}

func TestConvertToPodMetrics(t *testing.T) {
	src := &metricsv1beta1.PodMetrics{
		ObjectMeta: metav1.ObjectMeta{Name: TestPodName, Namespace: TestNamespace},
	}
	got := metricsutils.ConvertToPodMetrics(src)
	if got.PodName != TestPodName || got.Namespace != TestNamespace ||
		len(got.Containers) != constants.EmptySliceLength {
		t.Errorf(ExpectedConvertToPodMetrics, got, TestPodName, TestNamespace)
	}
}
