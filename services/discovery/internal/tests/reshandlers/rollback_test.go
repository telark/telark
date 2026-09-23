package reshandlers

import (
	"bytes"
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
	applicationmodel "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/config"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/coordination"
	"github.com/telark/discovery/internal/handlers/resources/applications"
	"github.com/telark/discovery/internal/tests/testutil"
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
	t.Cleanup(func() { applications.SetCoordinationBundle(nil, "") })
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
	applications.SetCoordinationBundle(nil, "")
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
	t.Cleanup(func() { applications.SetCoordinationBundle(nil, "") })
	mr.Close()

	testutil.Equal(t, "trigger with redis down", triggerRollback(shopApp).Code, http.StatusServiceUnavailable)
}

func triggerRollbackTo(name string, gen int) *httptest.ResponseRecorder {
	body := fmt.Sprintf(`{"snapshotGeneration":%d,"triggeredBy":"tester"}`, gen)
	return triggerRollbackWithBody(name, strings.NewReader(body))
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// The exporter host is a fixed cluster DNS name, so the stored application is
// served by swapping the default transport the rest client dials through.
func stubStoredApplication(t *testing.T, app applicationmodel.Application) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"status": http.StatusOK, "data": app})
	if err != nil {
		t.Fatal(err)
	}
	prev := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = prev })
}

// The snapshot stamped with the current generation is the pre-image of the
// latest change, so it is the newest valid target; a generation above the
// current one, or one with no stored snapshot, has nothing to apply.
func TestTriggerRollbackAcceptsCurrentGenerationRejectsFutureOrMissing(t *testing.T) {
	applications.SetCoordinationBundle(nil, "")
	stubStoredApplication(t, applicationmodel.Application{
		Name: shopApp,
		Snapshots: []applicationmodel.ApplicationSnapshot{
			{Generation: 1, ID: "snap-1"},
			{Generation: 3, ID: "snap-3"},
		},
		History: applicationmodel.ApplicationHistory{Generation: 3},
	})

	current := triggerRollbackTo(shopApp, constants.ThreeValue)
	testutil.Equal(t, "target equals current", current.Code, http.StatusOK)

	future := triggerRollbackTo(shopApp, futureGeneration)
	testutil.Equal(t, "target above current", future.Code, http.StatusBadRequest)
	testutil.Equal(t, "target above current message",
		strings.Contains(future.Body.String(), fmt.Sprintf(string(constants.ErrRollbackTargetNotOlder), constants.ThreeValue)), true)

	missing := triggerRollbackTo(shopApp, constants.TwoValue)
	testutil.Equal(t, "target with no snapshot", missing.Code, http.StatusBadRequest)
	testutil.Equal(t, "target with no snapshot message",
		strings.Contains(missing.Body.String(), fmt.Sprintf(string(constants.ErrRollbackSnapshotMissing), constants.TwoValue)), true)
}

const crdApplicationPath = "../../../../../charts/telark-crds/templates/crds/resources/application.yaml"

// The API server validates spec.rollbacks[].status against the CRD enum, so a
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
