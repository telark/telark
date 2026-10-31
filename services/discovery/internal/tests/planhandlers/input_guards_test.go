package planhandlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"
	xwareredis "github.com/telark/telark/internal/x-ware/redis/stream"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/coordination"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection"
	handlers "github.com/telark/telark/services/discovery/internal/handlers/plans/protection"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

func reasonBody(t *testing.T, reason string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]string{constants.FieldReason: reason})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// A cancel reason is at most 500 characters with no control character, refused with 400 before the plan lock; one
// that passes reaches the lock, held here, so it answers 409.
func TestCancelReasonIsBounded(t *testing.T) {
	mr := testutil.RedisEnv(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	handlers.SetCoordinationBundle(&coordination.CoordinationBundle{Lock: xwareredis.NewLockClient(rdb)})
	t.Cleanup(func() { handlers.SetCoordinationBundle(nil) })
	handlers.InitService(protection.NewService(nil, nil, nil, nil, nil, nil, nil, nil))
	t.Cleanup(func() { handlers.InitService(nil) })
	if err := mr.Set(constants.KeyPrefixLockPlanDecision+lockedPlanID, heldBy); err != nil {
		t.Fatal(err)
	}
	atLimit := strings.Repeat("r", constants.ReasonMaxLen)
	cases := []struct {
		name string
		body string
		want int
	}{
		{"no body", constants.EmptyString, http.StatusConflict},
		{"under the limit", reasonBody(t, atLimit[constants.DefaultAddValue:]), http.StatusConflict},
		{"at the limit", reasonBody(t, "  "+atLimit+"  "), http.StatusConflict},
		{"over the limit", reasonBody(t, atLimit+"r"), http.StatusBadRequest},
		{"control character", reasonBody(t, "stop\x07now"), http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handlers.Cancel(rec, planRequest(http.MethodPost, c.body))
			testutil.Equal(t, "status", rec.Code, c.want)
			if c.want == http.StatusBadRequest {
				testutil.Equal(t, "names the field", strings.Contains(rec.Body.String(), constants.FieldReason), true)
			}
		})
	}
}
