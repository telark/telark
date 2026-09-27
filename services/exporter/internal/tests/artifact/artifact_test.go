package artifact

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/utils/artifact"
)

const (
	targetName     = "report.json"
	tempGlob       = "*.tmp"
	lockKey        = "exporter:test:gc"
	otherLockKey   = "exporter:test:other"
	lockTTL        = time.Minute
	testDirPerm    = 0o750
	writeOKPayload = "{}"
)

var errSentinel = errors.New("writer failed")

func tempFiles(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, tempGlob))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	return matches
}

func assertNoTempLeft(t *testing.T, dir string) {
	t.Helper()
	if left := tempFiles(t, dir); len(left) != constants.DefaultInitValue {
		t.Fatalf("temp files left behind: %v", left)
	}
}

func writeOK(w io.Writer) error {
	_, err := io.WriteString(w, writeOKPayload)
	return err
}

func TestWriteAtomicWriterErrorRemovesTempAndReportsStage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, targetName)

	stage, err := artifact.WriteAtomic(path, func(io.Writer) error { return errSentinel })
	if stage != artifact.StageTempWrite {
		t.Fatalf("stage = %v, want StageTempWrite", stage)
	}
	if !errors.Is(err, errSentinel) {
		t.Fatalf("err = %v, want raw sentinel", err)
	}
	assertNoTempLeft(t, dir)
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("target must not exist after a failed write, stat: %v", statErr)
	}
}

func TestWriteAtomicRenameOntoNonEmptyDirReportsRenameStage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, targetName)
	if err := os.MkdirAll(filepath.Join(path, "child"), testDirPerm); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	stage, err := artifact.WriteAtomic(path, writeOK)
	if stage != artifact.StageRename {
		t.Fatalf("stage = %v, want StageRename", stage)
	}
	var linkErr *os.LinkError
	if !errors.As(err, &linkErr) {
		t.Fatalf("err = %T (%v), want raw *os.LinkError", err, err)
	}
	assertNoTempLeft(t, dir)
}

func TestWriteAtomicTempNameMatchesSweepSuffix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, targetName)

	stage, err := artifact.WriteAtomic(path, func(w io.Writer) error {
		matches := tempFiles(t, dir)
		if len(matches) != 1 {
			t.Fatalf("temp files during write = %v, want exactly one", matches)
		}
		if !strings.HasPrefix(filepath.Base(matches[0]), targetName) {
			t.Fatalf("temp name %q does not start with %q", filepath.Base(matches[0]), targetName)
		}
		return writeOK(w)
	})
	if err != nil || stage != artifact.StageNone {
		t.Fatalf("WriteAtomic = (%v, %v), want (StageNone, nil)", stage, err)
	}
	body, readErr := os.ReadFile(filepath.Clean(path))
	if readErr != nil || string(body) != writeOKPayload {
		t.Fatalf("target = %q (%v), want %q", body, readErr, writeOKPayload)
	}
	assertNoTempLeft(t, dir)
}

func TestIsSafeSegment(t *testing.T) {
	cases := map[string]bool{
		"":                     false,
		".":                    false,
		"..":                   false,
		"a/b":                  false,
		"../x":                 false,
		"a":                    true,
		"20260101T000000Z-end": true,
	}
	for id, want := range cases {
		if got := artifact.IsSafeSegment(id); got != want {
			t.Errorf("IsSafeSegment(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestTickAllowedNilRedisAllowsAndCancelledCtxDenies(t *testing.T) {
	if !artifact.TickAllowed(context.Background(), nil, lockKey, lockTTL) {
		t.Fatal("nil redis with a live ctx must allow the tick")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if artifact.TickAllowed(ctx, nil, lockKey, lockTTL) {
		t.Fatal("canceled ctx must deny the tick")
	}
}

func TestTickAllowedSecondCallerWithinTTLIsDenied(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	if !artifact.TickAllowed(ctx, client, lockKey, lockTTL) {
		t.Fatal("first caller must acquire the tick")
	}
	if artifact.TickAllowed(ctx, client, lockKey, lockTTL) {
		t.Fatal("second caller within the TTL must be denied")
	}
	if !artifact.TickAllowed(ctx, client, otherLockKey, lockTTL) {
		t.Fatal("a different key must be independent")
	}
}
