package notifstream

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	notifhandler "github.com/telark/telark/services/exporter/internal/handlers/notifications"
	notifstorage "github.com/telark/telark/services/exporter/internal/redis/notifications"
)

const (
	redisDown = "LOADING Redis is loading the dataset in memory"
	// The retry after a failed read, then one blocking read that finds nothing.
	recoverWait = 10 * time.Second
	burst       = 2
	noWrites    = 0
)

// After a Redis outage the hub resumes, and wakes every open stream once: entries may have been trimmed meanwhile.
func TestHubWakesEveryStreamAfterAnOutage(t *testing.T) {
	// Without Redis there is nothing to read: Run returns at once.
	notifstorage.NewHub(nil).Run(context.Background())

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	hub := notifstorage.NewHub(client)
	changes, stop, err := hub.Subscribe(testUserID)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	seed := &redis.XAddArgs{Stream: changedKey, Values: map[string]any{changedField: otherUserID}}
	if err := client.XAdd(context.Background(), seed).Err(); err != nil {
		t.Fatal(err)
	}
	mr.SetError(redisDown)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		hub.Run(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		_ = client.Close()
		<-done
	})

	time.Sleep(quietFor)
	mr.SetError("")
	select {
	case <-changes:
	case <-time.After(recoverWait):
		t.Fatal("no wake-up after the outage")
	}
}

// The channel holds one wake-up, so a burst of changes reaches the stream as one.
func TestABurstOfChangesWakesOnce(t *testing.T) {
	client, _ := setup(t)
	hub := runHub(t, client.Options().Addr)
	changes, stop, err := hub.Subscribe(testUserID)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	change := &redis.XAddArgs{Stream: changedKey, Values: map[string]any{changedField: testUserID}}
	for range burst {
		if err := client.XAdd(context.Background(), change).Err(); err != nil {
			t.Fatal(err)
		}
	}
	waitUntilReading(t, hub, client)
	if got := len(changes); got != oneEntry {
		t.Fatalf("pending wake-ups = %d, want %d", got, oneEntry)
	}
}

var errClientGone = errors.New("client gone")

// Takes this many writes, then fails every one, as a connection whose client left.
type leavingWriter struct {
	*httptest.ResponseRecorder
	writes int
}

func (w *leavingWriter) Write(p []byte) (int, error) {
	if w.writes == noWrites {
		return noWrites, errClientGone
	}
	w.writes--
	return w.ResponseRecorder.Write(p)
}

// The recorder's own WriteString would bypass Write.
func (w *leavingWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

// A client that left ends its stream at the first frame it can't take: the connected frame, or a ping.
func TestAStreamEndsWhenItsClientLeaves(t *testing.T) {
	client, _ := setup(t)
	handler := notifhandler.Events(notifstorage.NewHub(client), stubResolver{}, shortInterval, never)
	for name, writes := range map[string]int{"the connected frame": noWrites, "a ping": oneEntry} {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r = r.WithContext(xauthz.WithIdentity(r.Context(), xauthz.Identity{UserID: testUserID}))
			done := make(chan struct{})
			go func() {
				handler(&leavingWriter{ResponseRecorder: httptest.NewRecorder(), writes: writes}, r)
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(waitFor):
				t.Fatal("the stream outlived its client")
			}
		})
	}
}
