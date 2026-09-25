package k8sclient

import (
	"testing"

	"github.com/telark/kcore/k8sclient"
)

// A dedicated client reads its budget like the shared one: unset or unusable
// values fall back to the caller's defaults.
func TestRateLimitFromEnv(t *testing.T) {
	env := k8sclient.RateLimitEnv{
		QPSVar:       TestQPSVar,
		BurstVar:     TestBurstVar,
		DefaultQPS:   TestDefaultQPS,
		DefaultBurst: TestDefaultBurst,
	}
	cases := []struct {
		qps, burst string
		wantQPS    float32
		wantBurst  int
	}{
		{"", "", TestDefaultQPS, TestDefaultBurst},
		{"30", "60", TestEnvQPS, TestEnvBurst},
		{"nope", "0", TestDefaultQPS, TestDefaultBurst},
	}
	for _, c := range cases {
		t.Setenv(env.QPSVar, c.qps)
		t.Setenv(env.BurstVar, c.burst)
		qps, burst := k8sclient.RateLimitFromEnv(env)
		if qps != c.wantQPS || burst != c.wantBurst {
			t.Errorf(ExpectedRateLimitFromEnv, c.qps, c.burst, qps, burst, c.wantQPS, c.wantBurst)
		}
	}
}
