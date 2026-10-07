package prewarm

import (
	"testing"
	"testing/synctest"
	"time"

	natscore "github.com/telark/telark/internal/x-ware/nats/core"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/discovery/prewarm"
)

// The informer flush and the coordination jobs build these options on every run: a NATS
// that cannot be reached held each of them up to 30 s in a dial retry loop, even with
// NATS_HOST unset. Without a client the options come back at once and publishing is skipped.
func TestBuildOptionsDoNotWaitForUnreachableNATS(t *testing.T) {
	tests := []struct {
		name string
		host string
	}{
		{"NATS not configured", constants.EmptyString},
		{"NATS dial failing", "nats.invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(natscore.EnvNatsHost, tt.host)
			t.Setenv(natscore.EnvNatsUser, constants.EmptyString)
			synctest.Test(t, func(t *testing.T) {
				start := time.Now()
				for range constants.ThreeValue {
					if opts := prewarm.BuildPrewarmApplicationOptions(); opts.NatsClient != nil {
						t.Fatal("got a NATS client")
					}
				}
				if waited := time.Since(start); waited > 0 {
					t.Fatalf("waited %v for NATS", waited)
				}
			})
		})
	}
}
