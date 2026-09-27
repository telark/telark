package notifications

import (
	"strings"
	"testing"

	"github.com/telark/exporter/internal/constants"
	notiftypes "github.com/telark/exporter/internal/types/notifications"
	notifdiff "github.com/telark/exporter/internal/utils/notifications"
)

const (
	idA = "a"
	idB = "b"
	idC = "c"

	rolesKey = "roles"
	idsKey   = "ids"

	wantTwoEntries = 2
)

func ptr(s string) *string { return &s }

func TestDiffStringSlices(t *testing.T) {
	added, removed := notifdiff.DiffStringSlices([]string{idA, idB}, []string{idB, idC})
	if len(added) != constants.DefaultIncrementValue || added[constants.DefaultInitValue] != idC {
		t.Errorf("added = %v, want [c]", added)
	}
	if len(removed) != constants.DefaultIncrementValue || removed[constants.DefaultInitValue] != idA {
		t.Errorf("removed = %v, want [a]", removed)
	}
}

func TestDiffPtrStringSlices(t *testing.T) {
	old := []*string{ptr(idA), nil, ptr(idB)}
	next := []*string{ptr(idB), ptr(idC), nil}
	added, removed := notifdiff.DiffPtrStringSlices(old, next)
	if len(added) != constants.DefaultIncrementValue || added[constants.DefaultInitValue] != idC {
		t.Errorf("added = %v, want [c]", added)
	}
	if len(removed) != constants.DefaultIncrementValue || removed[constants.DefaultInitValue] != idA {
		t.Errorf("removed = %v, want [a]", removed)
	}
}

func TestExtractNewRoleIDsFromBody(t *testing.T) {
	got := notifdiff.ExtractNewRoleIDsFromBody(map[string]any{rolesKey: []any{"r1", 2, "r2"}}, rolesKey)
	if len(got) != wantTwoEntries || *got[constants.DefaultInitValue] != "r1" || *got[constants.DefaultIncrementValue] != "r2" {
		t.Errorf("got %v non-string entries not skipped", got)
	}
	if notifdiff.ExtractNewRoleIDsFromBody(map[string]any{}, rolesKey) != nil {
		t.Error("absent key should return nil")
	}
	if notifdiff.ExtractNewRoleIDsFromBody(map[string]any{rolesKey: "x"}, rolesKey) != nil {
		t.Error("non-slice value should return nil")
	}
}

func TestExtractNewStringIDsFromBody(t *testing.T) {
	if got := notifdiff.ExtractNewStringIDsFromBody(map[string]any{idsKey: []string{idA}}, idsKey); len(got) != constants.DefaultIncrementValue {
		t.Errorf("[]string not returned: %v", got)
	}
	got := notifdiff.ExtractNewStringIDsFromBody(map[string]any{idsKey: []any{idA, 1}}, idsKey)
	if len(got) != constants.DefaultIncrementValue || got[constants.DefaultInitValue] != idA {
		t.Errorf("[]any not filtered: %v", got)
	}
	if notifdiff.ExtractNewStringIDsFromBody(map[string]any{idsKey: 5}, idsKey) != nil {
		t.Error("unsupported type should return nil")
	}
}

func TestValidateForEmit(t *testing.T) {
	valid := notiftypes.Notification{UserID: "u", Type: "t", Title: "hi", Message: "m", Severity: notiftypes.SeverityInfo}
	if err := notiftypes.ValidateForEmit(&valid); err != nil {
		t.Errorf("valid notification rejected: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(n *notiftypes.Notification)
	}{
		{"no user", func(n *notiftypes.Notification) { n.UserID = constants.EmptyString }},
		{"no type", func(n *notiftypes.Notification) { n.Type = constants.EmptyString }},
		{"no title", func(n *notiftypes.Notification) { n.Title = constants.EmptyString }},
		{"no message", func(n *notiftypes.Notification) { n.Message = constants.EmptyString }},
		{"bad severity", func(n *notiftypes.Notification) { n.Severity = "nope" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := valid
			tt.mutate(&n)
			if err := notiftypes.ValidateForEmit(&n); err == nil {
				t.Errorf("%s: expected validation error", tt.name)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	n := notiftypes.Notification{
		Title:   strings.Repeat("t", notiftypes.MaxTitleLen+10),
		Message: strings.Repeat("m", notiftypes.MaxMessageLen+10),
	}
	notiftypes.Truncate(&n)
	if len(n.Title) != notiftypes.MaxTitleLen {
		t.Errorf("title len = %d, want %d", len(n.Title), notiftypes.MaxTitleLen)
	}
	if len(n.Message) != notiftypes.MaxMessageLen {
		t.Errorf("message len = %d, want %d", len(n.Message), notiftypes.MaxMessageLen)
	}
}
