package core

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/services/discovery/internal/constants"
)

// Sorted, so a republish keeps the order instead of Go's random map order.
func setToSlice(m map[string]bool) []string {
	out := slices.AppendSeq(make([]string, constants.DefaultInitValue, len(m)), maps.Keys(m))
	slices.Sort(out)
	return out
}

func intSetToSlice(m map[int]bool) []int {
	out := slices.AppendSeq(make([]int, constants.DefaultInitValue, len(m)), maps.Keys(m))
	slices.Sort(out)
	return out
}

func formatAppTime(t time.Time) string {
	if t.IsZero() {
		return constants.EmptyString
	}
	return t.UTC().Format(time.RFC3339)
}

func replaceSnapshotsWithExplicitTakenAt(m map[string]any, app *application.Application) {
	if m == nil || app == nil || len(app.Snapshots) == constants.DefaultInitValue {
		return
	}
	nowTaken := time.Now().UTC().Format(time.RFC3339Nano)
	out := make([]any, constants.DefaultInitValue, len(app.Snapshots))
	for i := range app.Snapshots {
		snap := app.Snapshots[i]
		taken := strings.TrimSpace(snap.TakenAt)
		if taken == constants.EmptyString {
			taken = nowTaken
			snap.TakenAt = taken
		}
		sb, err := json.Marshal(snap)
		if err != nil {
			continue
		}
		var sm map[string]any
		if err := json.Unmarshal(sb, &sm); err != nil {
			continue
		}
		sm[payloadKeyTakenAt] = taken
		out = append(out, sm)
	}
	m[payloadKeySnapshots] = out
}
