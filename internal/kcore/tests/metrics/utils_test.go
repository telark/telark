package metrics

import (
	"testing"

	"github.com/plsyro/kcore-pkg/metrics/metricsutils"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func TestFormatCPU(t *testing.T) {
	cases := []struct {
		input int64
	}{
		{TestCPUValue}, {1000}, {2500},
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
		{1024}, {1048576}, {TestMemoryValueGi},
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
		{"2", 2000},
		{TestEmptyString, 0},
		{TestInvalidCPU, 0},
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
		{TestEmptyString, 0},
		{TestInvalidMemory, 0},
	}
	for _, c := range cases {
		if got := metricsutils.ParseMemory(c.input); got != c.expect {
			t.Errorf(ExpectedParseMemoryReturned, c.input, got, c.expect)
		}
	}
}

func TestConvertToPodMetrics(t *testing.T) {
	pm := &metricsv1beta1.PodMetrics{}
	_ = metricsutils.ConvertToPodMetrics(pm)
}
