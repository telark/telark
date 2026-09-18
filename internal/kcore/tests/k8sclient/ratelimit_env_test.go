package k8sclient

import (
	"testing"

	"github.com/telark/kcore/k8sclient"
)

// A dedicated client reads its budget like the shared one: unset or unusable
// values fall back to the caller's defaults.
func TestRateLimitFromEnv(t *testing.T) {
	env := k8sclient.RateLimitEnv{QPSVar: "TEST_RL_QPS", BurstVar: "TEST_RL_BURST", DefaultQPS: 7, DefaultBurst: 14}
	cases := []struct {
		qps, burst string
		wantQPS    float32
		wantBurst  int
	}{
		{"", "", 7, 14},
		{"30", "60", 30, 60},
		{"nope", "0", 7, 14},
	}
	for _, c := range cases {
		t.Setenv(env.QPSVar, c.qps)
		t.Setenv(env.BurstVar, c.burst)
		qps, burst := k8sclient.RateLimitFromEnv(env)
		if qps != c.wantQPS || burst != c.wantBurst {
			t.Errorf("(%q,%q) = (%v,%d), want (%v,%d)", c.qps, c.burst, qps, burst, c.wantQPS, c.wantBurst)
		}
	}
}
