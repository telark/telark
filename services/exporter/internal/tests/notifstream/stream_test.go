package notifstream

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	dataconstants "github.com/telark/telark/internal/data/constants"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	notifhandler "github.com/telark/telark/services/exporter/internal/handlers/notifications"
	exprdb "github.com/telark/telark/services/exporter/internal/redis"
	notifstorage "github.com/telark/telark/services/exporter/internal/redis/notifications"
	notiftypes "github.com/telark/telark/services/exporter/internal/types/notifications"
)

const (
	testUserID   = "u1"
	otherUserID  = "u2"
	probeUserID  = "probe"
	testToken    = "st-1"
	testTarget   = "tgt"
	changedKey   = "notif:changed"
	changedField = "u"

	frameConnected = ": connected\n"
	frameChanged   = "event: changed\ndata: {}\n"
	framePing      = ": ping\n"

	oneEntry      = 1
	newestEntry   = 0
	never         = time.Hour
	shortInterval = 20 * time.Millisecond
	serverTimeout = 200 * time.Millisecond
	waitFor       = 2 * time.Second
	quietFor      = 200 * time.Millisecond
	frameBuffer   = 16
)

type stubResolver struct {
	revoked *atomic.Bool
	outage  bool
}

type stubError struct{}

func (stubError) Error() string { return "stub outage" }

func (s stubResolver) UserIDForToken(string) (string, error) {
	switch {
	case s.outage:
		return dataconstants.EmptyString, stubError{}
	case s.revoked != nil && s.revoked.Load():
		return dataconstants.EmptyString, xauthz.ErrNotFound
	}
	return testUserID, nil
}

func (stubResolver) GrantsForUser(string) (xauthz.Grants, error) {
	return xauthz.Grants{}, nil
}

func setup(t *testing.T) (*redis.Client, *notifstorage.Storage) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	exprdb.Set(client)
	storage, err := notifstorage.NewStorage()
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	return client, storage
}

// Each hub reads on its own connection, so closing it ends the blocking read at once.
func runHub(t *testing.T, addr string) *notifstorage.Hub {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: addr})
	hub := notifstorage.NewHub(client)
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
	waitUntilReading(t, hub, client)
	return hub
}

// Run starts at the stream's newest entry, so an entry written before its first read is never seen.
func waitUntilReading(t *testing.T, hub *notifstorage.Hub, client *redis.Client) {
	t.Helper()
	probe, stop, err := hub.Subscribe(probeUserID)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	deadline := time.After(waitFor)
	for {
		probeEntry := &redis.XAddArgs{Stream: changedKey, Values: map[string]any{changedField: probeUserID}}
		if err := client.XAdd(context.Background(), probeEntry).Err(); err != nil {
			t.Fatal(err)
		}
		select {
		case <-probe:
			return
		case <-deadline:
			t.Fatal("hub never started reading")
		case <-time.After(shortInterval):
		}
	}
}

func notification() notiftypes.Notification {
	return notiftypes.Notification{
		UserID:   testUserID,
		Type:     notiftypes.TypeRoleChanged,
		Title:    "title",
		Message:  "message",
		Severity: notiftypes.SeverityInfo,
		Metadata: map[string]any{notiftypes.MetaKeyTargetID: testTarget},
	}
}

func expectSignal(t *testing.T, changes <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-changes:
	case <-time.After(waitFor):
		t.Fatalf("%s: no signal", what)
	}
}

// Every write that changes a user's list adds one entry naming that user, which every hub reads.
func TestEveryWritePathSignals(t *testing.T) {
	ctx := context.Background()
	client, storage := setup(t)
	var created *notiftypes.Notification
	steps := []struct {
		name  string
		write func() error
	}{
		{"create", func() (err error) { created, err = storage.Emit(ctx, notification()); return err }},
		{"re-publish of an unread one", func() error { _, err := storage.Emit(ctx, notification()); return err }},
		{"mark read", func() error { return storage.MarkRead(ctx, testUserID, created.ID) }},
		{"delete", func() error { return storage.Delete(ctx, testUserID, created.ID) }},
		{"mark all read", func() error { return storage.MarkAllRead(ctx, testUserID) }},
		{"clear", func() error { return storage.Clear(ctx, testUserID) }},
	}
	for _, step := range steps {
		before := client.XLen(ctx, changedKey).Val()
		if err := step.write(); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		entries, err := client.XRevRangeN(ctx, changedKey, "+", "-", 1).Result()
		if err != nil || client.XLen(ctx, changedKey).Val() != before+oneEntry || len(entries) != oneEntry {
			t.Fatalf("%s: change stream did not grow by one entry (err %v)", step.name, err)
		}
		if got := entries[newestEntry].Values[changedField]; got != testUserID {
			t.Errorf("%s: entry names %v, want %s", step.name, got, testUserID)
		}
	}
}

// Two replicas each run a hub: a write on either reaches the user's streams on both, and nobody else's.
func TestTwoHubsBothSignal(t *testing.T) {
	mr := miniredis.RunT(t)
	writer := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = writer.Close() })
	exprdb.Set(writer)
	storage, err := notifstorage.NewStorage()
	if err != nil {
		t.Fatal(err)
	}
	first, second := runHub(t, mr.Addr()), runHub(t, mr.Addr())
	onFirst, stopFirst, err := first.Subscribe(testUserID)
	if err != nil {
		t.Fatal(err)
	}
	defer stopFirst()
	onSecond, stopSecond, err := second.Subscribe(testUserID)
	if err != nil {
		t.Fatal(err)
	}
	defer stopSecond()
	other, stopOther, err := second.Subscribe(otherUserID)
	if err != nil {
		t.Fatal(err)
	}
	defer stopOther()

	if _, err := storage.Emit(context.Background(), notification()); err != nil {
		t.Fatal(err)
	}
	expectSignal(t, onFirst, "first hub")
	expectSignal(t, onSecond, "second hub")
	select {
	case <-other:
		t.Error("another user's stream was woken")
	case <-time.After(quietFor):
	}
}

func TestSubscribeLimitsAndClose(t *testing.T) {
	_, _ = setup(t)
	hub := notifstorage.NewHub(exprdb.Get())
	for range notifstorage.StreamsPerUser {
		if _, _, err := hub.Subscribe(testUserID); err != nil {
			t.Fatalf("subscribe within the limit: %v", err)
		}
	}
	if _, _, err := hub.Subscribe(testUserID); err != notifstorage.ErrTooManyStreams {
		t.Errorf("one stream past the limit: err %v, want ErrTooManyStreams", err)
	}
	other, _, err := hub.Subscribe(otherUserID)
	if err != nil {
		t.Fatalf("another user's limit is their own: %v", err)
	}
	hub.Close()
	if _, open := <-other; open {
		t.Error("Close left a stream open")
	}
	if _, _, err := hub.Subscribe(otherUserID); err != notifstorage.ErrHubClosed {
		t.Errorf("subscribe after Close: err %v, want ErrHubClosed", err)
	}
	if _, _, err := notifstorage.NewHub(nil).Subscribe(testUserID); err != notifstorage.ErrRedisUnavailable {
		t.Errorf("subscribe without Redis: err %v, want ErrRedisUnavailable", err)
	}
}

func asUser(identity xauthz.Identity, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(xauthz.WithIdentity(r.Context(), identity)))
	})
}

func TestEventsRefusals(t *testing.T) {
	_, _ = setup(t)
	full := notifstorage.NewHub(exprdb.Get())
	for range notifstorage.StreamsPerUser {
		if _, _, err := full.Subscribe(testUserID); err != nil {
			t.Fatal(err)
		}
	}
	closed := notifstorage.NewHub(exprdb.Get())
	closed.Close()

	tests := []struct {
		name     string
		hub      *notifstorage.Hub
		identity xauthz.Identity
		want     int
	}{
		{"internal caller", notifstorage.NewHub(exprdb.Get()), xauthz.Identity{UserID: testUserID, Internal: true}, http.StatusForbidden},
		{"no user", notifstorage.NewHub(exprdb.Get()), xauthz.Identity{}, http.StatusForbidden},
		{"ninth stream", full, xauthz.Identity{UserID: testUserID}, http.StatusTooManyRequests},
		{"shutting down", closed, xauthz.Identity{UserID: testUserID}, http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := asUser(tt.identity, notifhandler.Events(tt.hub, stubResolver{}, never, never))
			rec := httptest.NewRecorder()
			// Bounded, so a refusal that regressed into a stream fails here instead of hanging.
			ctx, cancel := context.WithTimeout(context.Background(), quietFor)
			defer cancel()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/notifications/events", nil).WithContext(ctx))
			if rec.Code != tt.want {
				t.Errorf("code = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

type streamServer struct {
	server *httptest.Server
	hub    *notifstorage.Hub
}

func newStreamServer(t *testing.T, resolver stubResolver, ping, recheck time.Duration) streamServer {
	t.Helper()
	client, _ := setup(t)
	hub := runHub(t, client.Options().Addr)
	srv := httptest.NewUnstartedServer(asUser(xauthz.Identity{UserID: testUserID}, notifhandler.Events(hub, resolver, ping, recheck)))
	srv.Config.ReadTimeout = serverTimeout
	srv.Config.WriteTimeout = serverTimeout
	srv.Config.RegisterOnShutdown(hub.Close)
	srv.Start()
	t.Cleanup(srv.Close)
	return streamServer{server: srv, hub: hub}
}

// Frames arrive on the channel, without their closing blank line; it closes when the stream ends.
func openStream(t *testing.T, srv *httptest.Server) (<-chan string, http.Header) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(dataconstants.HeaderSessionToken, testToken)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	frames := make(chan string, frameBuffer)
	go readFrames(resp.Body, frames)
	return frames, resp.Header
}

func readFrames(body io.Reader, frames chan<- string) {
	defer close(frames)
	reader := bufio.NewReader(body)
	var frame strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		if line == "\n" {
			frames <- frame.String()
			frame.Reset()
			continue
		}
		_, _ = frame.WriteString(line)
	}
}

func expectFrame(t *testing.T, frames <-chan string, want string) {
	t.Helper()
	deadline := time.After(waitFor)
	for {
		select {
		case got, open := <-frames:
			if !open {
				t.Fatalf("stream ended before %q", want)
			}
			if got == want {
				return
			}
		case <-deadline:
			t.Fatalf("no %q frame", want)
		}
	}
}

func expectEnd(t *testing.T, frames <-chan string) {
	t.Helper()
	deadline := time.After(waitFor)
	for {
		select {
		case _, open := <-frames:
			if !open {
				return
			}
		case <-deadline:
			t.Fatal("the stream stayed open")
		}
	}
}

func TestStreamOutlivesServerTimeouts(t *testing.T) {
	s := newStreamServer(t, stubResolver{}, never, never)
	frames, header := openStream(t, s.server)
	if got := header.Get("Content-Type"); got != "text/event-stream" || header.Get("X-Accel-Buffering") != "no" {
		t.Errorf("headers: Content-Type %q, X-Accel-Buffering %q", got, header.Get("X-Accel-Buffering"))
	}
	expectFrame(t, frames, frameConnected)

	time.Sleep(serverTimeout + serverTimeout)
	storage, err := notifstorage.NewStorage()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Emit(context.Background(), notification()); err != nil {
		t.Fatal(err)
	}
	expectFrame(t, frames, frameChanged)
}

func TestStreamPings(t *testing.T) {
	s := newStreamServer(t, stubResolver{}, shortInterval, never)
	frames, _ := openStream(t, s.server)
	expectFrame(t, frames, frameConnected)
	expectFrame(t, frames, framePing)
}

func TestRevokedSessionEndsStream(t *testing.T) {
	revoked := &atomic.Bool{}
	s := newStreamServer(t, stubResolver{revoked: revoked}, never, shortInterval)
	frames, _ := openStream(t, s.server)
	expectFrame(t, frames, frameConnected)
	revoked.Store(true)
	expectEnd(t, frames)
}

// The session backend being unreachable is not a revocation.
func TestOutageKeepsStream(t *testing.T) {
	s := newStreamServer(t, stubResolver{outage: true}, shortInterval, shortInterval)
	frames, _ := openStream(t, s.server)
	expectFrame(t, frames, frameConnected)
	time.Sleep(quietFor)
	// Pings buffered before an end would still read: only a ping sent after the rechecks counts.
	for drained := false; !drained; {
		select {
		case _, open := <-frames:
			if !open {
				t.Fatal("an outage ended the stream")
			}
		default:
			drained = true
		}
	}
	expectFrame(t, frames, framePing)
}

func TestShutdownClosesStream(t *testing.T) {
	s := newStreamServer(t, stubResolver{}, never, never)
	frames, _ := openStream(t, s.server)
	expectFrame(t, frames, frameConnected)

	ctx, cancel := context.WithTimeout(context.Background(), waitFor)
	defer cancel()
	if err := s.server.Config.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown waited on the open stream: %v", err)
	}
	expectEnd(t, frames)
}
