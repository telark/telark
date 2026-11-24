package authrequests

import (
	"strings"
	"testing"
	"time"

	"github.com/telark/telark/services/exporter/internal/constants"
	envmanager "github.com/telark/telark/services/exporter/internal/managers/envs"
)

const (
	day              = 24 * time.Hour
	defaultRetention = 30 * day
	weekRetention    = 7 * day
)

// Empty means 30 days; anything but a whole number of days, at least 1, stops the exporter and names the variable.
func TestRetentionFromTheEnvironment(t *testing.T) {
	t.Cleanup(func() { _ = envmanager.InitAuthRequestRetention() })
	cases := []struct {
		value string
		want  time.Duration
		fails bool
	}{
		{constants.EmptyString, defaultRetention, false},
		{" 7 ", weekRetention, false},
		{"1", day, false},
		{"0", none, true},
		{"-3", none, true},
		{"1.5", none, true},
		{"a month", none, true},
	}
	for _, c := range cases {
		t.Run(c.value, func(t *testing.T) {
			t.Setenv(constants.AuthRequestRetentionDaysEnv, c.value)
			err := envmanager.InitAuthRequestRetention()
			equal(t, "refused", err != nil, c.fails)
			if c.fails {
				equal(t, "names the variable", strings.Contains(err.Error(), constants.AuthRequestRetentionDaysEnv), true)
				return
			}
			equal(t, "retention", envmanager.GetAuthRequestRetention(), c.want)
		})
	}
}
