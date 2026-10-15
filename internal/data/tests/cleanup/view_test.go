package cleanup

import (
	"testing"

	"github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/data/resources/shared"
)

const (
	cleanupFinalizer = "telark.io/cleanup"
	otherFinalizer   = "example.com/other"
	deletionStamp    = "2026-10-01T00:00:00Z"
)

// A record is released only while it is being deleted and still holds the finalizer, so a missing
// view or an empty deletion timestamp reads as neither.
func TestCleanupView(t *testing.T) {
	stamp, blank := deletionStamp, constants.EmptyString
	cases := []struct {
		name         string
		view         *shared.CleanupView
		deleting     bool
		hasFinalizer bool
	}{
		{"no view", nil, false, false},
		{"live, no finalizer", &shared.CleanupView{}, false, false},
		{"blank timestamp", &shared.CleanupView{DeletionTimestamp: &blank, Finalizers: []string{cleanupFinalizer}}, false, true},
		{"deleting, other finalizer", &shared.CleanupView{DeletionTimestamp: &stamp, Finalizers: []string{otherFinalizer}}, true, false},
		{"deleting, finalizer held", &shared.CleanupView{DeletionTimestamp: &stamp, Finalizers: []string{cleanupFinalizer}}, true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.view.IsDeleting(); got != c.deleting {
				t.Errorf("IsDeleting = %v, want %v", got, c.deleting)
			}
			if got := c.view.HasFinalizer(cleanupFinalizer); got != c.hasFinalizer {
				t.Errorf("HasFinalizer = %v, want %v", got, c.hasFinalizer)
			}
		})
	}
}
