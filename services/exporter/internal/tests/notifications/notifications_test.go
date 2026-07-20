package notifications

import (
	"strings"
	"testing"

	notiftypes "github.com/telark/exporter/internal/types/notifications"
	notifdiff "github.com/telark/exporter/internal/utils/notifications"
)

func ptr(s string) *string { return &s }

func TestDiffStringSlices(t *testing.T) {
	added, removed := notifdiff.DiffStringSlices([]string{"a", "b"}, []string{"b", "c"})
	if len(added) != 1 || added[0] != "c" {
		t.Errorf("added = %v, want [c]", added)
	}
	if len(removed) != 1 || removed[0] != "a" {
		t.Errorf("removed = %v, want [a]", removed)
	}
}

func TestDiffPtrStringSlices(t *testing.T) {
	old := []*string{ptr("a"), nil, ptr("b")}
	next := []*string{ptr("b"), ptr("c"), nil}
	added, removed := notifdiff.DiffPtrStringSlices(old, next)
	if len(added) != 1 || added[0] != "c" {
		t.Errorf("added = %v, want [c]", added)
	}
	if len(removed) != 1 || removed[0] != "a" {
		t.Errorf("removed = %v, want [a]", removed)
	}
}

func TestExtractNewRoleIDsFromBody(t *testing.T) {
	got := notifdiff.ExtractNewRoleIDsFromBody(map[string]any{"roles": []any{"r1", 2, "r2"}}, "roles")
	if len(got) != 2 || *got[0] != "r1" || *got[1] != "r2" {
		t.Errorf("got %v non-string entries not skipped", got)
	}
	if notifdiff.ExtractNewRoleIDsFromBody(map[string]any{}, "roles") != nil {
		t.Error("absent key should return nil")
	}
	if notifdiff.ExtractNewRoleIDsFromBody(map[string]any{"roles": "x"}, "roles") != nil {
		t.Error("non-slice value should return nil")
	}
}

func TestExtractNewStringIDsFromBody(t *testing.T) {
	if got := notifdiff.ExtractNewStringIDsFromBody(map[string]any{"ids": []string{"a"}}, "ids"); len(got) != 1 {
		t.Errorf("[]string not returned: %v", got)
	}
	got := notifdiff.ExtractNewStringIDsFromBody(map[string]any{"ids": []any{"a", 1}}, "ids")
	if len(got) != 1 || got[0] != "a" {
		t.Errorf("[]any not filtered: %v", got)
	}
	if notifdiff.ExtractNewStringIDsFromBody(map[string]any{"ids": 5}, "ids") != nil {
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
		{"no user", func(n *notiftypes.Notification) { n.UserID = "" }},
		{"no type", func(n *notiftypes.Notification) { n.Type = "" }},
		{"no title", func(n *notiftypes.Notification) { n.Title = "" }},
		{"no message", func(n *notiftypes.Notification) { n.Message = "" }},
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
