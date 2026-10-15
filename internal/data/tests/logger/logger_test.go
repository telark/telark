package logger

import (
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/data/logger"
)

// Loggers made inside write bind the swapped stdout, so a line that skips stderr is caught too.
func capture(t *testing.T, write func()) (stdout, stderr string) {
	t.Helper()
	outReader, outWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errReader, errWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prevOut, prevErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outWriter, errWriter
	write()
	os.Stdout, os.Stderr = prevOut, prevErr
	for _, writer := range []*os.File{outWriter, errWriter} {
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
	}
	printed, err := io.ReadAll(outReader)
	if err != nil {
		t.Fatal(err)
	}
	logged, err := io.ReadAll(errReader)
	if err != nil {
		t.Fatal(err)
	}
	return string(printed), string(logged)
}

// UseCommandOutput can't be undone in a process, so a rerun (-count) checks only the command case.
var commandOutputOn bool

// A service logs every level on stdout. Once a command takes stdout for its answer, its log lines move to
// stderr and its debug lines go nowhere. The switch is one-way, so the service case runs first.
func TestCommandOutput(t *testing.T) {
	levels := []struct {
		tag       string
		write     func(*logger.CustomLogger, string)
		inCommand bool
	}{
		{"[INFO] ", (*logger.CustomLogger).Info, true},
		{"[WARNING] ", (*logger.CustomLogger).Warn, true},
		{"[ERROR] ", (*logger.CustomLogger).Error, true},
		{"[DEBUG] ", (*logger.CustomLogger).Debug, false},
	}
	writeAll := func() {
		lg := logger.NewCustomLogger("test: ")
		for _, level := range levels {
			level.write(lg, level.tag)
		}
	}

	if !commandOutputOn {
		stdout, stderr := capture(t, writeAll)
		if stderr != "" {
			t.Fatalf("service stderr = %q, want nothing", stderr)
		}
		for _, level := range levels {
			if !strings.Contains(stdout, level.tag) {
				t.Fatalf("service stdout %q lacks %q", stdout, level.tag)
			}
		}
	}

	logger.UseCommandOutput()
	commandOutputOn = true
	stdout, stderr := capture(t, writeAll)
	if stdout != "" {
		t.Fatalf("command stdout = %q, want nothing", stdout)
	}
	for _, level := range levels {
		if strings.Contains(stderr, level.tag) != level.inCommand {
			t.Fatalf("command stderr %q: has %q = %v, want %v", stderr, level.tag, !level.inCommand, level.inCommand)
		}
	}
}

const (
	sharedPrefix     = "[shared] "
	otherPrefix      = "[other] "
	concurrentPrefix = "[concurrent] "
	concurrentCalls  = 8
)

// Every caller that asks for a prefix gets the one logger made for it, even when they ask at once.
func TestGetLoggerSharesOneLoggerPerPrefix(t *testing.T) {
	first := logger.GetLogger(sharedPrefix)
	if logger.GetLogger(sharedPrefix) != first {
		t.Fatal("a second call for the same prefix made a new logger")
	}
	if logger.GetLogger(otherPrefix) == first {
		t.Fatal("another prefix got the same logger")
	}
	got := make([]*logger.CustomLogger, concurrentCalls)
	var wg sync.WaitGroup
	for i := range got {
		wg.Go(func() { got[i] = logger.GetLogger(concurrentPrefix) })
	}
	wg.Wait()
	for i, lg := range got {
		if lg != got[constants.DefaultInitValue] {
			t.Fatalf("concurrent call %d got its own logger", i)
		}
	}
}
