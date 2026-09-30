package reshandlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	applicationmodel "github.com/telark/telark/internal/data/resources/application"
	notifclient "github.com/telark/telark/internal/rest/clients/notifications"
	"github.com/telark/telark/services/discovery/internal/config"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/coordination"
	"github.com/telark/telark/services/discovery/internal/handlers/resources/applications"
	"github.com/telark/telark/services/discovery/internal/helpers/async"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	shopApp          = "shop"
	replicaID        = "replica-1"
	futureGeneration = 4
)

func triggerRollback(name string) *httptest.ResponseRecorder {
	return triggerRollbackWithBody(name, http.NoBody)
}

func triggerRollbackWithBody(name string, body io.Reader) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", body)
	applications.TriggerRollback(rec, mux.SetURLVars(req, map[string]string{constants.NameParam: name}))
	return rec
}

// The handler takes its lock before reading the body, so a body that parks on
// its first Read pins the handler inside the locked section until released.
type blockingBody struct {
	reached chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *blockingBody) Read([]byte) (int, error) {
	b.once.Do(func() { close(b.reached) })
	<-b.release
	return constants.DefaultInitValue, io.EOF
}

// All replicas share one Redis key per application, so a trigger is refused
// with 409 while another holder has it and gets past the lock once released.
// An empty body stops the handler at 422 right after the lock, before the
// exporter is consulted, which is enough to show the lock was acquired and
// then released by the handler itself.
func TestTriggerRollbackRejectsSecondHolder(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	bundle := coordination.NewCoordinationBundle(rdb, replicaID, config.LoadCoordinationConfig())
	applications.SetCoordinationBundle(bundle, replicaID)
	t.Cleanup(func() { applications.SetCoordinationBundle(nil, constants.EmptyString) })
	ctx := context.Background()
	key := constants.KeyPrefixLockRollback + shopApp

	testutil.Equal(t, "first trigger", triggerRollback(shopApp).Code, http.StatusUnprocessableEntity)
	testutil.Equal(t, "handler released its lock", triggerRollback(shopApp).Code, http.StatusUnprocessableEntity)

	held, err := bundle.Lock.Acquire(ctx, key, "replica-2", constants.DefaultLockTTL)
	if err != nil {
		t.Fatal(err)
	}
	testutil.Equal(t, "held by other replica", held, true)
	testutil.Equal(t, "trigger while held", triggerRollback(shopApp).Code, http.StatusConflict)

	if err := bundle.Lock.Release(ctx, key, "replica-2"); err != nil {
		t.Fatal(err)
	}
	testutil.Equal(t, "trigger after release", triggerRollback(shopApp).Code, http.StatusUnprocessableEntity)
}

// Without a bundle the routes still serve, so a per-process mutex must carry
// the serialization: a trigger parked mid-request holds the app and a second
// one is refused with 409 until the first finishes.
func TestTriggerRollbackWithoutBundleSerializesPerProcess(t *testing.T) {
	applications.SetCoordinationBundle(nil, constants.EmptyString)
	body := &blockingBody{reached: make(chan struct{}), release: make(chan struct{})}
	done := make(chan int)
	go func() { done <- triggerRollbackWithBody("cart", body).Code }()
	<-body.reached

	testutil.Equal(t, "second trigger while first holds", triggerRollback("cart").Code, http.StatusConflict)
	close(body.release)
	testutil.Equal(t, "first trigger", <-done, http.StatusUnprocessableEntity)
	testutil.Equal(t, "trigger after release", triggerRollback("cart").Code, http.StatusUnprocessableEntity)
}

// A Redis outage is not a rollback in flight: it must surface as 503 so the
// caller retries instead of hunting for a rollback that does not exist.
func TestTriggerRollbackRedisDownIs503(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	bundle := coordination.NewCoordinationBundle(rdb, replicaID, config.LoadCoordinationConfig())
	applications.SetCoordinationBundle(bundle, replicaID)
	t.Cleanup(func() { applications.SetCoordinationBundle(nil, constants.EmptyString) })
	mr.Close()

	testutil.Equal(t, "trigger with redis down", triggerRollback(shopApp).Code, http.StatusServiceUnavailable)
}

const (
	callerID          = "u-caller"
	spoofedID         = "u-spoofed"
	pendingRollbackID = "rb-pending"
)

type patchedApplication struct {
	Rollbacks []applicationmodel.RollbackEntry `json:"rollbacks"`
	Spec      map[string]any                   `json:"spec"`
}

func triggerRollbackTo(gen int) *httptest.ResponseRecorder {
	return triggerRollbackAs(shopApp, gen, callerID)
}

// The body still carries triggeredBy, as older clients send it; the handler must ignore it.
func triggerRollbackAs(name string, gen int, userID string) *httptest.ResponseRecorder {
	body := fmt.Sprintf(`{"snapshotGeneration":%d,"triggeredBy":%q}`, gen, spoofedID)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	if userID != constants.EmptyString {
		req.Header.Set(constants.HeaderUserID, userID)
	}
	applications.TriggerRollback(rec, mux.SetURLVars(req, map[string]string{constants.NameParam: name}))
	return rec
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Serves the stored application for every exporter call and hands back the
// bodies the handler patched, so a test can read what it wrote.
func stubStoredApplication(t *testing.T, app applicationmodel.Application) *[][]byte {
	t.Helper()
	patches := &[][]byte{}
	stubExporter(t, func(r *http.Request) *http.Response {
		if r.Body != nil && r.Method != http.MethodGet {
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			*patches = append(*patches, raw)
		}
		return jsonResponse(t, http.StatusOK, storedAnswer(t, app))
	})
	return patches
}

// The snapshot stamped with the current generation is the pre-image of the
// latest change, so it is the newest valid target; a generation above the
// current one, or one with no stored snapshot, has nothing to apply.
func TestTriggerRollbackAcceptsCurrentGenerationRejectsFutureOrMissing(t *testing.T) {
	applications.SetCoordinationBundle(nil, constants.EmptyString)
	stubStoredApplication(t, applicationmodel.Application{
		Name: shopApp,
		Snapshots: []applicationmodel.ApplicationSnapshot{
			{Generation: 1, ID: "snap-1"},
			{Generation: 3, ID: "snap-3"},
		},
		History: applicationmodel.ApplicationHistory{Generation: 3},
	})

	current := triggerRollbackTo(constants.ThreeValue)
	testutil.Equal(t, "target equals current", current.Code, http.StatusOK)

	future := triggerRollbackTo(futureGeneration)
	testutil.Equal(t, "target above current", future.Code, http.StatusBadRequest)
	testutil.Equal(t, "target above current message",
		strings.Contains(future.Body.String(), fmt.Sprintf(string(constants.ErrRollbackTargetNotOlder), constants.ThreeValue)), true)

	missing := triggerRollbackTo(constants.TwoValue)
	testutil.Equal(t, "target with no snapshot", missing.Code, http.StatusBadRequest)
	testutil.Equal(t, "target with no snapshot message",
		strings.Contains(missing.Body.String(), fmt.Sprintf(string(constants.ErrRollbackSnapshotMissing), constants.TwoValue)), true)
}

// A generation whose file for one of the app's namespaces was pruned restored the
// other namespace only and reported success; the trigger refuses it with a reason.
func TestTriggerRollbackRefusesGenerationMissingANamespace(t *testing.T) {
	applications.SetCoordinationBundle(nil, constants.EmptyString)
	stubStoredApplication(t, applicationmodel.Application{
		Name:       shopApp,
		Namespaces: applicationmodel.Namespaces{Items: []applicationmodel.NamespaceEntry{{Name: "ns1"}, {Name: "ns2"}}},
		Snapshots: []applicationmodel.ApplicationSnapshot{
			{Generation: constants.TwoValue, ID: "snap-2-ns1", Namespace: "ns1"},
			{Generation: constants.TwoValue, ID: "snap-2-ns2", Namespace: "ns2"},
			{Generation: constants.ThreeValue, ID: "snap-3-ns2", Namespace: "ns2"},
		},
		History: applicationmodel.ApplicationHistory{Generation: constants.ThreeValue},
	})
	partial := triggerRollbackTo(constants.ThreeValue)
	testutil.Equal(t, "half generation refused", partial.Code, http.StatusBadRequest)
	testutil.Equal(t, "reason names the namespace",
		strings.Contains(partial.Body.String(), fmt.Sprintf(string(constants.ErrRollbackSnapshotIncomplete), constants.ThreeValue, "ns1")), true)
	testutil.Equal(t, "whole generation accepted", triggerRollbackTo(constants.TwoValue).Code, http.StatusOK)
}

// Any caller allowed to roll back could attribute the rollback (its audit entry
// and its notification) to another user through the body; the record names the
// verified caller and a request without one is refused.
func TestTriggerRollbackTakesTriggeredByFromCaller(t *testing.T) {
	applications.SetCoordinationBundle(nil, constants.EmptyString)
	patches := stubStoredApplication(t, applicationmodel.Application{
		Name:      shopApp,
		Snapshots: []applicationmodel.ApplicationSnapshot{{Generation: constants.ThreeValue, ID: "snap-3"}},
		History:   applicationmodel.ApplicationHistory{Generation: constants.ThreeValue},
	})

	testutil.Equal(t, "no caller", triggerRollbackAs(shopApp, constants.ThreeValue, constants.EmptyString).Code, http.StatusBadRequest)
	testutil.Equal(t, "nothing patched", len(*patches), constants.DefaultInitValue)

	testutil.Equal(t, "caller", triggerRollbackAs(shopApp, constants.ThreeValue, callerID).Code, http.StatusOK)
	testutil.Equal(t, "one patch", len(*patches), constants.DefaultAddValue)
	var patched patchedApplication
	if err := json.Unmarshal((*patches)[constants.DefaultInitValue], &patched); err != nil {
		t.Fatal(err)
	}
	testutil.Equal(t, "rollbacks sent as a view key, not under spec", patched.Spec == nil, true)
	testutil.Equal(t, "one entry", len(patched.Rollbacks), constants.DefaultAddValue)
	testutil.Equal(t, "triggeredBy", patched.Rollbacks[constants.DefaultInitValue].TriggeredBy, callerID)
}

// The abort notification's applicationId held the rollback id, so a consumer that
// resolves the application from it found nothing.
func TestAbortRollbackNotifiesWithApplicationName(t *testing.T) {
	applications.SetCoordinationBundle(nil, constants.EmptyString)
	async.Init()
	bodies := stubStoredApplication(t, applicationmodel.Application{
		Name:      shopApp,
		Rollbacks: []applicationmodel.RollbackEntry{{ID: pendingRollbackID, Status: constants.RollbackStatusPending}},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, constants.PathSeparator, http.NoBody)
	req.Header.Set(constants.HeaderUserID, callerID)
	applications.AbortRollback(rec, mux.SetURLVars(req, map[string]string{
		constants.NameParam:           shopApp,
		constants.RollbackIDPathParam: pendingRollbackID,
	}))
	testutil.Equal(t, "abort", rec.Code, http.StatusOK)

	async.Drain()
	testutil.Equal(t, "patch then notification", len(*bodies), constants.TwoValue)
	var sent notifclient.Notification
	if err := json.Unmarshal((*bodies)[constants.DefaultAddValue], &sent); err != nil {
		t.Fatal(err)
	}
	testutil.Equal(t, "applicationId", sent.Metadata[notifclient.MetaKeyApplicationID], any(shopApp))
	testutil.Equal(t, "targetId", sent.Metadata[notifclient.MetaKeyTargetID], any(pendingRollbackID))
}

const crdApplicationPath = "../../../../../charts/telark-crds/templates/crds/applications.yaml"

// The API server validates status.rollbacks[].status against the CRD enum, so a
// status this service writes but the enum omits makes the patch unpersistable:
// the abort handler answered 500 and the entry stayed pending.
func TestRollbackStatusesAreAcceptedByCRDEnum(t *testing.T) {
	crd, err := os.ReadFile(crdApplicationPath)
	if err != nil {
		t.Fatal(err)
	}
	var enumLine string
	for _, line := range strings.Split(string(crd), "\n") {
		if strings.Contains(line, "enum:") && strings.Contains(line, constants.RollbackStatusInProgress) {
			enumLine = line
			break
		}
	}
	testutil.Equal(t, "rollback status enum found", enumLine != constants.EmptyString, true)

	for _, status := range []string{
		constants.RollbackStatusPending,
		constants.RollbackStatusInProgress,
		constants.RollbackStatusSuccess,
		constants.RollbackStatusFailed,
		constants.RollbackStatusAborted,
	} {
		testutil.Equal(t, "enum allows "+status, strings.Contains(enumLine, status), true)
	}
}
