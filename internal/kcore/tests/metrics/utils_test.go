package metrics_test

import (
	"testing"

	"github.com/plsyro/kcore-pkg/metrics/utils"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func TestFormatCPU(t *testing.T) {
	cases := []struct {
		input int64
	}{
		{500}, {1000}, {2500},
	}
	for _, c := range cases {
		if got := utils.FormatCPU(c.input); got == "" {
			t.Errorf("FormatCPU(%d) returned empty string", c.input)
		}
	}
}

func TestFormatMemory(t *testing.T) {
	cases := []struct {
		input int64
	}{
		{1024}, {1048576}, {1073741824},
	}
	for _, c := range cases {
		if got := utils.FormatMemory(c.input); got == "" {
			t.Errorf("FormatMemory(%d) returned empty string", c.input)
		}
	}
}

func TestParseCPU(t *testing.T) {
	cases := []struct {
		input  string
		expect int64
	}{
		{"500m", 500},
		{"2", 2000},
		{"", 0},
		{"notanumber", 0},
	}
	for _, c := range cases {
		if got := utils.ParseCPU(c.input); got != c.expect {
			t.Errorf("ParseCPU(%q) = %d, want %d", c.input, got, c.expect)
		}
	}
}

func TestParseMemory(t *testing.T) {
	cases := []struct {
		input  string
		expect int64
	}{
		{"1Gi", 1073741824},
		{"512Mi", 536870912},
		{"", 0},
		{"notanumber", 0},
	}
	for _, c := range cases {
		if got := utils.ParseMemory(c.input); got != c.expect {
			t.Errorf("ParseMemory(%q) = %d, want %d", c.input, got, c.expect)
		}
	}
}

func TestConvertToPodMetrics(t *testing.T) {
	pm := &metricsv1beta1.PodMetrics{}
	_ = utils.ConvertToPodMetrics(pm) // Just ensure it doesn't panic
}
