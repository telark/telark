package historyshared

import (
	"testing"

	"github.com/telark/discovery/internal/core/applications/history/shared"
	"github.com/telark/discovery/internal/tests/testutil"
)

// Only the four known severities are valid.
func TestIsValidSnapshotSeverity(t *testing.T) {
	for _, s := range []string{shared.SeverityCritical, shared.SeverityHigh, shared.SeverityMedium, shared.SeverityLow} {
		testutil.Equal(t, "valid "+s, shared.IsValidSnapshotSeverity(s), true)
	}
	testutil.Equal(t, "unknown", shared.IsValidSnapshotSeverity("bogus"), false)
	testutil.Equal(t, "empty", shared.IsValidSnapshotSeverity(""), false)
}

// SnapshotPayloadSize reports a positive size only for a payload carrying a
// non-empty resources list, and false for every degenerate shape.
func TestSnapshotPayloadSize(t *testing.T) {
	if _, ok := shared.SnapshotPayloadSize(map[string]any{}); ok {
		t.Fatal("missing resources key should be false")
	}
	if _, ok := shared.SnapshotPayloadSize(map[string]any{shared.PayloadKeyResources: []map[string]any{}}); ok {
		t.Fatal("empty []map resources should be false")
	}
	if _, ok := shared.SnapshotPayloadSize(map[string]any{shared.PayloadKeyResources: []any{}}); ok {
		t.Fatal("empty []any resources should be false")
	}
	if _, ok := shared.SnapshotPayloadSize(map[string]any{shared.PayloadKeyResources: "nope"}); ok {
		t.Fatal("wrong-typed resources should be false")
	}
	size, ok := shared.SnapshotPayloadSize(map[string]any{
		shared.PayloadKeyResources: []map[string]any{{"kind": "Deployment"}},
	})
	if !ok || size <= 0 {
		t.Fatalf("non-empty resources = (%d,%v), want positive size", size, ok)
	}
}
