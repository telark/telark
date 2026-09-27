package planhandlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/coordination"
	"github.com/telark/discovery/internal/core/plans/protection"
	handlers "github.com/telark/discovery/internal/handlers/plans/protection"
	"github.com/telark/discovery/internal/tests/testutil"
	xwareredis "github.com/telark/x-ware/redis/stream"
)

const (
	lockedPlanID = "plan-locked"
	heldBy       = "another-writer"
)

func strptr(s string) *string { return &s }

func planRequest(method, body string) *http.Request {
	req := httptest.NewRequest(method, "/", strings.NewReader(body))
	req.Header.Set(constants.HeaderUserID, "u-1")
	return mux.SetURLVars(req, map[string]string{constants.IDPathParam: lockedPlanID})
}

// The service has no exporter, so any answer other than a body error means the decode let
// the request through.
func TestPlanBodiesAreBoundedAndStrict(t *testing.T) {
	handlers.InitService(protection.NewService(nil, nil, nil, nil, nil, nil, nil, nil))
	t.Cleanup(func() { handlers.InitService(nil) })
	oversized := `{"name":"` + strings.Repeat("a", constants.MaxRequestBodyBytes) + `"}`
	cases := []struct {
		name string
		body string
		want int
	}{
		{"unknown field", `{"name":"p","approvalGate":"off"}`, http.StatusBadRequest},
		{"oversized", oversized, http.StatusRequestEntityTooLarge},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handlers.Prepare(rec, planRequest(http.MethodPost, c.body))
			testutil.Equal(t, "status", rec.Code, c.want)
		})
	}
}

// Clear and Reactivate raced Decide, Update and Cancel without the per-plan lock and could leave
// enforce policies behind with no owning plan.
func TestClearAndReactivateTakeThePlanLock(t *testing.T) {
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

	cases := []struct {
		name    string
		handler http.HandlerFunc
		method  string
	}{
		{"clear", handlers.Clear, http.MethodDelete},
		{"reactivate", handlers.Reactivate, http.MethodPost},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c.handler(rec, planRequest(c.method, ""))
			testutil.Equal(t, "status", rec.Code, http.StatusConflict)
		})
	}
}

// The lock outlives the operation budget and is released by its holder only.
func TestHoldLockTTLAndRelease(t *testing.T) {
	mr := testutil.RedisEnv(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	client := xwareredis.NewLockClient(rdb)
	key := constants.KeyPrefixLockPlanDecision + lockedPlanID
	acquired, err := client.Acquire(t.Context(), key, heldBy, constants.PlanLockTTL)
	testutil.Equal(t, "acquire", err, nil)
	testutil.Equal(t, "acquired", acquired, true)
	testutil.Equal(t, "ttl above budget", mr.TTL(key) > constants.ProtectionPlanDeployTimeout, true)

	release := protection.HoldLock(client, key, heldBy)
	mr.FastForward(constants.ProtectionPlanDeployTimeout + time.Second)
	testutil.Equal(t, "held past budget", mr.Exists(key), true)
	release()
	testutil.Equal(t, "released", mr.Exists(key), false)
}
