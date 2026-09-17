package base

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	shared "github.com/telark/data/resources/shared"
	"github.com/telark/notifier/internal/subscribers/base"
	"github.com/telark/notifier/internal/tests/testutil"
	"github.com/telark/rest/response"
	natscore "github.com/telark/x-ware/nats/core"
	natstreams "github.com/telark/x-ware/nats/streams"
)

const (
	appSubject    = "telark.applications.update"
	deleteSubject = "telark.applications.delete"
	appScope      = string(shared.ApplicationSpecScope)
)

// ExecuteDeleteHandler must call the delete backend exactly when the message
// carries a resource name, and skip it otherwise — a stray delete on a missing
// name would target the wrong (or no) application CR.
func TestExecuteDeleteHandler(t *testing.T) {
	cases := []struct {
		name       string
		msg        *nats.Msg
		status     int
		wantCalled bool
		wantName   string
	}{
		{"no parsed header", testutil.Msg(deleteSubject, nil), http.StatusOK, false, ""},
		{"malformed header", testutil.HeaderMsg(deleteSubject, "{bad"), http.StatusOK, false, ""},
		{"missing resource name", testutil.Msg(deleteSubject, &natscore.Message{}), http.StatusOK, false, ""},
		{"backend error", testutil.Msg(deleteSubject, &natscore.Message{ResourceName: "app1"}), http.StatusInternalServerError, true, "app1"},
		{"success", testutil.Msg(deleteSubject, &natscore.Message{ResourceName: "app1"}), http.StatusOK, true, "app1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var called bool
			var gotName string
			df := func(name string) base.GenericResponse {
				called, gotName = true, name
				return testutil.Resp{Status: c.status, Message: "boom"}
			}
			_ = base.ExecuteDeleteHandler(c.msg, df, "deleted %s", "delete %s failed: %s")
			testutil.Equal(t, "called", called, c.wantCalled)
			testutil.Equal(t, "name", gotName, c.wantName)
		})
	}
}

// ExecuteUpdateHandler must reach the patch backend only for a well-formed
// update (parsed header, known scope, resolvable name); every malformed case is
// acked-and-dropped without a patch so a bad message never mutates a CR.
func TestExecuteUpdateHandler(t *testing.T) {
	valid := &natscore.Message{ResourceName: "app1", Scope: appScope, Data: map[string]any{"replicas": 2}}
	cases := []struct {
		name       string
		msg        *nats.Msg
		status     int
		wantCalled bool
	}{
		{"no parsed header", testutil.Msg(appSubject, nil), http.StatusOK, false},
		{"malformed header", testutil.HeaderMsg(appSubject, "{bad"), http.StatusOK, false},
		{"missing scope", testutil.Msg(appSubject, &natscore.Message{ResourceName: "app1"}), http.StatusOK, false},
		{"unknown scope", testutil.Msg(appSubject, &natscore.Message{ResourceName: "app1", Scope: "nope", Data: map[string]any{}}), http.StatusOK, false},
		{"backend error", testutil.Msg(appSubject, valid), http.StatusInternalServerError, true},
		{"success", testutil.Msg(appSubject, valid), http.StatusOK, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var called bool
			pf := func(_ string, _ map[string]any) base.GenericResponse {
				called = true
				return testutil.Resp{Status: c.status, Message: "boom"}
			}
			_ = base.ExecuteUpdateHandler(c.msg, "name", base.BuildPatchBodyFromScope, pf, "patched %s", "patch %s failed: %s")
			testutil.Equal(t, "called", called, c.wantCalled)
		})
	}
}

// SharedExecuteHandler routes by action and only forwards to the resource
// handler once the payload is parsed; a transform failure is swallowed (acked)
// so the message is not redelivered forever.
func TestSharedExecuteHandler(t *testing.T) {
	msg := func() *nats.Msg {
		return testutil.Msg(appSubject, &natscore.Message{Data: map[string]any{"k": "v"}})
	}
	cases := []struct {
		name        string
		msg         *nats.Msg
		action      natscore.Action
		transform   func([]byte) ([]byte, error)
		wantHandled bool
	}{
		{"delete forwards", msg(), natscore.Delete, nil, true},
		{"update passthrough", msg(), natscore.Update, nil, true},
		{"update transformed", msg(), natscore.Update, func(b []byte) ([]byte, error) { return b, nil }, true},
		{"transform error swallowed", msg(), natscore.Update, func(_ []byte) ([]byte, error) { return nil, errors.New("x") }, false},
		{"no parsed header", testutil.Msg(appSubject, nil), natscore.Update, nil, false},
		{"malformed header", testutil.HeaderMsg(appSubject, "{bad"), natscore.Update, nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var handled bool
			getHandler := func(_ natscore.Action) func(*nats.Msg) error {
				return func(_ *nats.Msg) error { handled = true; return nil }
			}
			_ = base.SharedExecuteHandler(c.msg, c.action, "app", getHandler, c.transform)
			testutil.Equal(t, "handled", handled, c.wantHandled)
		})
	}
}

// BuildPatchBodyFromScope wraps the CR spec and resolves the resource name from
// the message or the payload; an unknown scope or an unresolvable name is an
// error, never a silent empty patch.
func TestBuildPatchBodyFromScope(t *testing.T) {
	cases := []struct {
		name     string
		scope    string
		data     map[string]any
		fromMsg  string
		wantName string
		wantErr  bool
	}{
		{"name from message", appScope, map[string]any{}, "app1", "app1", false},
		{"name from data", appScope, map[string]any{"name": "app2"}, "", "app2", false},
		{"unknown scope", "nope", map[string]any{"name": "app2"}, "", "", true},
		{"unresolvable name", appScope, map[string]any{}, "", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body, name, err := base.BuildPatchBodyFromScope(c.scope, c.data, "name", c.fromMsg)
			testutil.Equal(t, "err", err != nil, c.wantErr)
			testutil.Equal(t, "name", name, c.wantName)
			if !c.wantErr {
				if _, ok := body["spec"]; !ok {
					t.Fatalf("patch body missing spec: %v", body)
				}
			}
		})
	}
}

// AckWithLog always attempts to ack; it returns the ack error only when the ack
// itself fails (there is no live subscription for a synthetic message here).
func TestAckWithLog(t *testing.T) {
	for _, c := range []struct {
		name    string
		logMsg  string
		isError bool
	}{
		{"info with message", "hello", false},
		{"error with message", "bad", true},
		{"no message", "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if err := base.AckWithLog(testutil.Msg(appSubject, nil), appSubject, c.logMsg, c.isError); err == nil {
				t.Fatalf("expected ack error for synthetic message")
			}
		})
	}
}

// HandleUnknown acks and never errors — unrecognised actions must not stall the
// consumer.
func TestHandleUnknown(t *testing.T) {
	s := base.NewBaseSubscriber(natscore.Applications, shared.Application, nil)
	if err := s.HandleUnknown(testutil.Msg(appSubject, nil)); err != nil {
		t.Fatalf("HandleUnknown = %v, want nil", err)
	}
}

// ValidateMessage skips JetStream acks, rejects empty/garbled payloads, and
// stamps the parsed-message header for valid ones.
func TestValidateMessage(t *testing.T) {
	s := base.NewBaseSubscriber(natscore.Applications, shared.Application, nil)
	valid := testutil.Data(&natscore.Message{ResourceName: "app1"})
	cases := []struct {
		name    string
		msg     *nats.Msg
		wantErr bool
	}{
		{"jetstream ack skipped", testutil.RawMsg("$JS.ACK.telark", nil), false},
		{"empty data", testutil.RawMsg(appSubject, nil), true},
		{"malformed data", testutil.RawMsg(appSubject, []byte("{bad")), true},
		{"valid", testutil.RawMsg(appSubject, valid), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testutil.Equal(t, "err", s.ValidateMessage(c.msg) != nil, c.wantErr)
		})
	}
}

// HandleMessage validates, de-duplicates within its window, extracts the action
// from the subject, and drives the handler callback with retries.
func TestHandleMessage(t *testing.T) {
	newSub := func(cb func(*nats.Msg, natscore.Action) error) *base.BaseSubscriber {
		s := base.NewBaseSubscriber(natscore.Applications, shared.Application, nil)
		s.RetryDelay = time.Millisecond
		s.MaxRetries = 2
		s.SetHandlerCallback(cb)
		return s
	}
	validMsg := func() *nats.Msg {
		return testutil.RawMsg(appSubject, testutil.Data(&natscore.Message{ResourceName: "app1"}))
	}

	t.Run("success invokes callback with action", func(t *testing.T) {
		var gotAction natscore.Action
		s := newSub(func(_ *nats.Msg, a natscore.Action) error { gotAction = a; return nil })
		if err := s.ProcessMessage(context.Background(), validMsg()); err != nil {
			t.Fatalf("ProcessMessage = %v", err)
		}
		testutil.Equal(t, "action", gotAction, natscore.Update)
	})

	t.Run("duplicate is skipped", func(t *testing.T) {
		var calls int
		s := newSub(func(_ *nats.Msg, _ natscore.Action) error { calls++; return nil })
		m := validMsg()
		_ = s.ProcessMessage(context.Background(), m)
		_ = s.ProcessMessage(context.Background(), m)
		testutil.Equal(t, "callback invocations", calls, 1)
	})

	t.Run("callback failure surfaces after retries", func(t *testing.T) {
		var calls int
		s := newSub(func(_ *nats.Msg, _ natscore.Action) error { calls++; return errors.New("fail") })
		if err := s.ProcessMessage(context.Background(), validMsg()); err == nil {
			t.Fatalf("expected error after exhausted retries")
		}
		testutil.Equal(t, "retry attempts", calls, 2)
	})

	t.Run("invalid message rejected before callback", func(t *testing.T) {
		var calls int
		s := newSub(func(_ *nats.Msg, _ natscore.Action) error { calls++; return nil })
		if err := s.ProcessMessage(context.Background(), testutil.RawMsg(appSubject, nil)); err == nil {
			t.Fatalf("expected validation error")
		}
		testutil.Equal(t, "callback invocations", calls, 0)
	})
}

// NewBaseSubscriber wires the resource type through to GetResourceType so the
// manager can key subscribers by the resource they own.
func TestNewBaseSubscriber(t *testing.T) {
	s := base.NewBaseSubscriber(natscore.Applications, shared.Application, nil)
	testutil.Equal(t, "resource type", s.GetResourceType(), shared.Application)
}

// GenericResponseAdapter must be nil-safe: a missing upstream response reads as
// a zero status and empty message, not a panic.
func TestGenericResponseAdapter(t *testing.T) {
	nilAdapter := &base.GenericResponseAdapter{}
	testutil.Equal(t, "nil status", nilAdapter.GetStatus(), 0)
	testutil.Equal(t, "nil message", nilAdapter.GetMessage(), "")

	set := &base.GenericResponseAdapter{Resp: &response.GenericResponse{Status: http.StatusOK, Message: "ok"}}
	testutil.Equal(t, "status", set.GetStatus(), http.StatusOK)
	testutil.Equal(t, "message", set.GetMessage(), "ok")
}

// Subscribe wires the create/update/delete consumers against a real JetStream
// and drives each fetched message through the handler — end-to-end proof that a
// published application event reaches the subscriber's callback.
func TestSubscribeAndProcess(t *testing.T) {
	c := testutil.NatsServer(t)
	if err := natstreams.CreateStreams(c); err != nil {
		t.Fatalf("create streams: %v", err)
	}
	s := base.NewBaseSubscriber(natscore.Applications, shared.Application, nil)

	processed := make(chan natscore.Action, 1)
	s.SetHandlerCallback(func(_ *nats.Msg, a natscore.Action) error {
		processed <- a
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	if err := s.SubscribeWithContext(ctx, c); err != nil {
		t.Fatalf("SubscribeWithContext: %v", err)
	}
	t.Cleanup(func() { cancel(); s.Drain() })

	payload := testutil.Data(&natscore.Message{
		ResourceName: "app1",
		ResourceType: shared.Application,
		Scope:        appScope,
		Data:         map[string]any{"replicas": 1},
	})
	if _, err := c.JetStream.Publish(natscore.GetTopicName(natscore.Applications, natscore.Update), payload); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case a := <-processed:
		testutil.Equal(t, "processed action", a, natscore.Update)
	case <-time.After(5 * time.Second):
		t.Fatal("published message was not processed")
	}
}

// The worker pool may interleave applications but must never reorder one
// application's messages, and Drain must return only once every fetched
// message has gone through the handler.
func TestWorkerPoolKeepsPerAppOrder(t *testing.T) {
	c := testutil.NatsServer(t)
	if err := natstreams.CreateStreams(c); err != nil {
		t.Fatalf("create streams: %v", err)
	}
	s := base.NewBaseSubscriber(natscore.Applications, shared.Application, nil)

	const perApp = 20
	apps := []string{"app-a", "app-b"}
	var mu sync.Mutex
	seen := map[string][]float64{}
	s.SetHandlerCallback(func(m *nats.Msg, _ natscore.Action) error {
		var msg natscore.Message
		if err := json.Unmarshal(m.Data, &msg); err != nil {
			return err
		}
		time.Sleep(10 * time.Millisecond) // keep the queues non-empty so Drain does the flushing
		mu.Lock()
		defer mu.Unlock()
		seen[msg.ResourceName] = append(seen[msg.ResourceName], msg.Data.(float64))
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	if err := s.SubscribeWithContext(ctx, c); err != nil {
		t.Fatalf("SubscribeWithContext: %v", err)
	}
	t.Cleanup(func() { cancel(); s.Drain() })

	topic := natscore.GetTopicName(natscore.Applications, natscore.Update)
	for i := range perApp {
		for _, app := range apps {
			payload := testutil.Data(&natscore.Message{ResourceName: app, Scope: appScope, Data: i})
			if _, err := c.JetStream.Publish(topic, payload); err != nil {
				t.Fatalf("publish: %v", err)
			}
		}
	}

	// Wait until JetStream has delivered everything to the fetch loop, so the
	// remaining work sits in the worker queues when Drain runs.
	consumer := natscore.GetConsumerName(natscore.Applications, natscore.GetQueueName(natscore.Applications, natscore.Update), topic)
	deadline := time.Now().Add(10 * time.Second)
	for {
		info, err := c.JetStream.ConsumerInfo(natscore.GetStreamName(natscore.Applications), consumer)
		if err != nil {
			t.Fatalf("consumer info: %v", err)
		}
		if info.NumPending == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d messages never delivered", info.NumPending)
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	s.Drain()

	mu.Lock()
	defer mu.Unlock()
	for _, app := range apps {
		testutil.Equal(t, app+" handled", len(seen[app]), perApp)
		for i, seq := range seen[app] {
			testutil.Equal(t, app+" order", seq, float64(i))
		}
	}
}

// jsMsg shapes a message the way a JetStream pull fetch hands it over: the ack
// reply carries the stream sequence the worker's stale-redelivery guard reads.
func jsMsg(app string, streamSeq uint64) *nats.Msg {
	m := testutil.RawMsg(appSubject, testutil.Data(&natscore.Message{ResourceName: app, Scope: appScope, Data: streamSeq}))
	m.Reply = fmt.Sprintf("$JS.ACK.stream.consumer.1.%d.%d.0.0", streamSeq, streamSeq)
	m.Sub = &nats.Subscription{}
	return m
}

// A1 is NAK'd on a transient failure, A2 applies while the redelivery is still
// pending, then the redelivered A1 queues on the same worker: it must be
// dropped, not applied as the final spec. The pool is fed directly because a
// real JetStream only redelivers after the NAK delay or AckWait, and A1 is
// NAK'd through NakWithLog as the resource handlers do — that clears the dedup
// entry, so only the guard stands between the copy and the handler.
func TestWorkerPoolDropsStaleRedelivery(t *testing.T) {
	c := testutil.NatsServer(t)
	if err := natstreams.CreateStreams(c); err != nil {
		t.Fatalf("create streams: %v", err)
	}
	s := base.NewBaseSubscriber(natscore.Applications, shared.Application, nil)

	applied := make(chan float64, 4)
	s.SetHandlerCallback(func(m *nats.Msg, _ natscore.Action) error {
		var msg natscore.Message
		if err := json.Unmarshal(m.Data, &msg); err != nil {
			return err
		}
		seq := msg.Data.(float64)
		applied <- seq
		if seq == 1 {
			_ = s.NakWithLog(m, m.Subject, "transient failure") // no server behind the synthetic message; only the dedup reset matters
		}
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	if err := s.SubscribeWithContext(ctx, c); err != nil {
		t.Fatalf("SubscribeWithContext: %v", err)
	}
	t.Cleanup(func() { cancel(); s.Drain() })

	s.Dispatch(jsMsg("app-a", 1))
	s.Dispatch(jsMsg("app-a", 2))
	s.Dispatch(jsMsg("app-a", 1)) // the redelivered copy
	s.Dispatch(jsMsg("app-a", 3)) // same worker, so its arrival proves the copy was already decided

	var got []float64
	for len(got) < 3 {
		select {
		case v := <-applied:
			got = append(got, v)
		case <-time.After(5 * time.Second):
			t.Fatalf("applied %v, want [1 2 3]", got)
		}
	}
	testutil.Equal(t, "applied", fmt.Sprint(got), "[1 2 3]")
}

// A durable consumer left by an older release carries the old AckWait and
// MaxAckPending; AddConsumer refuses the mismatch, so CreateConsumer must
// converge the existing consumer instead of failing the subscriber at boot.
func TestCreateConsumerConvergesExistingDurable(t *testing.T) {
	c := testutil.NatsServer(t)
	if err := natstreams.CreateStreams(c); err != nil {
		t.Fatalf("create streams: %v", err)
	}
	stream := natscore.GetStreamName(natscore.Applications)
	topic := natscore.GetTopicName(natscore.Applications, natscore.Update)
	queue := natscore.GetQueueName(natscore.Applications, natscore.Update)
	consumer := natscore.GetConsumerName(natscore.Applications, queue, topic)

	info, err := natstreams.CreateConsumer(c, stream, consumer, topic, queue)
	if err != nil {
		t.Fatalf("CreateConsumer: %v", err)
	}
	old := info.Config
	old.AckWait = time.Second
	if _, err := c.JetStream.UpdateConsumer(stream, &old); err != nil {
		t.Fatalf("UpdateConsumer: %v", err)
	}

	info, err = natstreams.CreateConsumer(c, stream, consumer, topic, queue)
	if err != nil {
		t.Fatalf("CreateConsumer on existing durable: %v", err)
	}
	testutil.Equal(t, "ack wait", info.Config.AckWait, natstreams.StreamAckWait)
}
