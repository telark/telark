package bootstrap

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/telark/telark/internal/data/resources/telarkconfig"
	"github.com/telark/telark/services/exporter/internal/bootstrap"
	"github.com/telark/telark/services/exporter/internal/constants"
	envmanager "github.com/telark/telark/services/exporter/internal/managers/envs"
)

const (
	sixHours    = "6"
	sixHourWait = 6 * time.Hour
	pastTheWait = 7 * time.Hour
	defaultWait = 24 * time.Hour
	// Two notices (the expiry and the new request) and four refusals.
	waitTexts = 6
)

// Empty means 24 hours; anything but a whole number of hours, at least 1, stops the exporter and names the variable.
func TestWaitFromTheEnvironment(t *testing.T) {
	t.Cleanup(func() { _ = envmanager.InitBreakGlassWait() })
	cases := []struct {
		value string
		want  time.Duration
		fails bool
	}{
		{constants.EmptyString, defaultWait, false},
		{" 6 ", sixHourWait, false},
		{"1", time.Hour, false},
		{"0", none, true},
		{"-3", none, true},
		{"1.5", none, true},
		{"a day", none, true},
	}
	for _, c := range cases {
		t.Run(c.value, func(t *testing.T) {
			t.Setenv(constants.BreakGlassWaitHoursEnv, c.value)
			err := envmanager.InitBreakGlassWait()
			equal(t, "refused", err != nil, c.fails)
			if c.fails {
				equal(t, "names the variable", strings.Contains(err.Error(), constants.BreakGlassWaitHoursEnv), true)
				return
			}
			equal(t, "wait", envmanager.GetBreakGlassWait(), c.want)
		})
	}
}

// A configured wait moves the mark, the status names it, and every refusal and notice that states the wait reads it:
// a recovery opened 7 hours ago, which the bootstrap account signed in during, expired and a new one opened.
func TestConfiguredWait(t *testing.T) {
	t.Cleanup(func() { _ = envmanager.InitBreakGlassWait() })
	t.Setenv(constants.BreakGlassWaitHoursEnv, sixHours)
	if err := envmanager.InitBreakGlassWait(); err != nil {
		t.Fatal(err)
	}
	withNotices(t)
	owner := active(pinnedID, pinnedEmail, true, adminRoleID)
	owner.lastLogin = signedInAt(time.Now().Add(-time.Hour))
	client := installFake(t, append(directory(t), configWith(t, pinned(pending(time.Now().Add(-pastTheWait)))), owner.seed(t))...)
	opened := open(t, pinnedEmail, http.StatusOK)
	requests := storedStatus(t, client).Requests
	equal(t, "expired", requests[one].Reason, telarkconfig.BootstrapReasonExpired)
	equal(t, "new request", opened.Request.ID != requestID, true)
	equal(t, "status waitHours", statusAs(t, adminSession(pinnedID)).WaitHours, int(sixHourWait/time.Hour))

	texts := make([]string, none, waitTexts)
	for _, notice := range notices(t, pinnedID) {
		texts = append(texts, notice.Message)
	}
	for _, err := range []error{bootstrap.ErrExpired, bootstrap.ErrNotEligible, bootstrap.ErrRecoveryApproval, bootstrap.ErrKeep} {
		texts = append(texts, err.Error())
	}
	equal(t, "texts", len(texts), waitTexts)
	for _, text := range texts {
		if !strings.Contains(text, "6 hours") && !strings.Contains(text, "6-hour") {
			t.Fatalf("%q doesn't state the 6-hour wait", text)
		}
	}
}
