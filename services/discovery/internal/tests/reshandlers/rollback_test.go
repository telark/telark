package reshandlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"github.com/telark/discovery/internal/config"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/coordination"
	"github.com/telark/discovery/internal/handlers/resources/applications"
	"github.com/telark/discovery/internal/tests/testutil"
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
	return 0, io.EOF
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
	bundle := coordination.NewCoordinationBundle(rdb, "replica-1", config.LoadCoordinationConfig())
	applications.SetCoordinationBundle(bundle, "replica-1")
	t.Cleanup(func() { applications.SetCoordinationBundle(nil, "") })
	ctx := context.Background()
	key := constants.KeyPrefixLockRollback + "shop"

	testutil.Equal(t, "first trigger", triggerRollback("shop").Code, http.StatusUnprocessableEntity)
	testutil.Equal(t, "handler released its lock", triggerRollback("shop").Code, http.StatusUnprocessableEntity)

	held, err := bundle.Lock.Acquire(ctx, key, "replica-2", constants.DefaultLockTTL)
	if err != nil {
		t.Fatal(err)
	}
	testutil.Equal(t, "held by other replica", held, true)
	testutil.Equal(t, "trigger while held", triggerRollback("shop").Code, http.StatusConflict)

	if err := bundle.Lock.Release(ctx, key, "replica-2"); err != nil {
		t.Fatal(err)
	}
	testutil.Equal(t, "trigger after release", triggerRollback("shop").Code, http.StatusUnprocessableEntity)
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
	bundle := coordination.NewCoordinationBundle(rdb, "replica-1", config.LoadCoordinationConfig())
	applications.SetCoordinationBundle(bundle, "replica-1")
	t.Cleanup(func() { applications.SetCoordinationBundle(nil, "") })
	mr.Close()

	testutil.Equal(t, "trigger with redis down", triggerRollback("shop").Code, http.StatusServiceUnavailable)
}
