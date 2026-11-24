package testutil

import (
	"bytes"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/telark/telark/cmd/telark/internal/constants"
	"github.com/telark/telark/cmd/telark/internal/root"
)

const (
	awaitTimeout = 10 * time.Second
	awaitPoll    = 10 * time.Millisecond
	awaitFormat  = "timed out waiting for %s"
	timedOut     = "timed out waiting for the command to end"
)

var ErrWrite = errors.New("write failed")

func Run(t *testing.T, args ...string) Result {
	t.Helper()
	var stdout bytes.Buffer
	return RunTo(t, &stdout, args...)
}

// A nil args slice would make cobra read os.Args, the test binary's own flags.
func RunTo(t *testing.T, stdout io.Writer, args ...string) Result {
	t.Helper()
	var stderr bytes.Buffer
	code := root.Run(t.Context(), stdout, &stderr, append([]string{}, args...))
	result := Result{Stderr: stderr.String(), Code: code}
	if buffer, ok := stdout.(*bytes.Buffer); ok {
		result.Stdout = buffer.String()
	}
	return result
}

// Start runs a command that waits on the cluster, so the test can change the cluster while it runs.
func Start(t *testing.T, stdout io.Writer, args ...string) <-chan Result {
	t.Helper()
	done := make(chan Result, oneObject)
	go func() { done <- RunTo(t, stdout, args...) }()
	return done
}

// Wait fails the test when the command doesn't end in time, rather than hang the run.
func Wait(t *testing.T, done <-chan Result) Result {
	t.Helper()
	select {
	case result := <-done:
		return result
	case <-time.After(awaitTimeout):
		t.Fatal(timedOut)
		return Result{}
	}
}

// Created waits for the command to create an object of the resource, and returns its name.
func (f *FakeCluster) Created(t *testing.T, resource string) string {
	t.Helper()
	var name string
	eventually(t, resource, func() bool {
		f.mutex.Lock()
		defer f.mutex.Unlock()
		if index := slices.IndexFunc(f.objects, func(object storedObject) bool { return object.resource == resource }); index != missingObject {
			name = f.objects[index].name
		}
		return name != ""
	})
	return name
}

// Watching waits for a watch on the resource, so the next change reaches the command as an event of its own.
func (f *FakeCluster) Watching(t *testing.T, resource string) {
	t.Helper()
	eventually(t, resource, func() bool {
		f.mutex.Lock()
		defer f.mutex.Unlock()
		return slices.ContainsFunc(f.watchers, func(subscriber *watcher) bool { return subscriber.path.resource == resource })
	})
}

func (s *Stream) Write(p []byte) (int, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.buffer.Write(p)
}

func (s *Stream) String() string {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.buffer.String()
}

// Await waits until the command has printed part.
func (s *Stream) Await(t *testing.T, part string) {
	t.Helper()
	eventually(t, part, func() bool { return strings.Contains(s.String(), part) })
}

func eventually(t *testing.T, what string, done func() bool) {
	t.Helper()
	for deadline := time.Now().Add(awaitTimeout); !done(); time.Sleep(awaitPoll) {
		if time.Now().After(deadline) {
			t.Fatalf(awaitFormat, what)
		}
	}
}

func (FailingWriter) Write([]byte) (int, error) {
	return constants.DefaultInitValue, ErrWrite
}

func Equal[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}

func Contains(t *testing.T, name, text, part string) {
	t.Helper()
	if !strings.Contains(text, part) {
		t.Errorf("%s: %q does not contain %q", name, text, part)
	}
}
