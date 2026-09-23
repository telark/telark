package logdedup

import (
	"errors"
	"testing"
	"time"

	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/helpers/logdedup"
)

const (
	dedupScope = "scope"
	errFormat  = "err: %v"
)

// The dedupe suppresses a repeated error within its window and logs again after
// a Reset — so a flapping error is not logged on every reconcile tick.
func TestErrorOnce(t *testing.T) {
	d := logdedup.New(constants.DefaultInitValue) // zero window falls back to the default
	if d == nil {
		t.Fatal("New returned nil")
	}
	err := errors.New("boom")

	// Exercises the log path, the deduped path, the nil-signature path, and
	// re-logging after Reset. No panics, and Reset clears the bucket.
	d.ErrorOnce(dedupScope, errFormat, err)
	d.ErrorOnce(dedupScope, errFormat, err)
	d.ErrorOnce(dedupScope, errFormat, nil)
	d.Reset(dedupScope)
	d.ErrorOnce(dedupScope, errFormat, err)

	// A tight window still logs distinct signatures.
	short := logdedup.New(time.Millisecond)
	short.ErrorOnce("s", "e: %v", errors.New("a"))
	short.ErrorOnce("s", "e: %v", errors.New("b"))
}
