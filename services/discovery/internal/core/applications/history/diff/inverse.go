package diff

import (
	"time"

	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/applications/history/changes"
	"github.com/telark/discovery/internal/core/applications/history/utils"
)

type changeKey struct {
	field string
	typ   string
	value string
}

func signatureOf(c application.ApplicationChange) changeKey {
	switch c.ChangeType {
	case changes.ChangeTypeRemoved:
		return changeKey{field: c.Field, typ: c.ChangeType, value: derefChangeValue(c.OldValue)}
	case changes.ChangeTypeAdded:
		return changeKey{field: c.Field, typ: c.ChangeType, value: derefChangeValue(c.NewValue)}
	case changes.ChangeTypeUpdated:
		return changeKey{
			field: c.Field,
			typ:   c.ChangeType,
			value: derefChangeValue(c.OldValue) + "|" + derefChangeValue(c.NewValue),
		}
	default:
		return changeKey{field: c.Field, typ: c.ChangeType}
	}
}

func inverseOf(c application.ApplicationChange) changeKey {
	switch c.ChangeType {
	case changes.ChangeTypeRemoved:
		return changeKey{field: c.Field, typ: changes.ChangeTypeAdded, value: derefChangeValue(c.OldValue)}
	case changes.ChangeTypeAdded:
		return changeKey{field: c.Field, typ: changes.ChangeTypeRemoved, value: derefChangeValue(c.NewValue)}
	case changes.ChangeTypeUpdated:
		return changeKey{
			field: c.Field,
			typ:   c.ChangeType,
			value: derefChangeValue(c.NewValue) + "|" + derefChangeValue(c.OldValue),
		}
	default:
		return changeKey{field: c.Field, typ: c.ChangeType}
	}
}

func isInversePair(
	prev application.ChangeLogEntry,
	newChanges []application.ApplicationChange,
	newClass string,
	newDetectedAt time.Time,
) bool {
	if prev.ChangeClass != newClass {
		return false
	}
	prevTime := utils.ParseRFC3339OrNano(prev.DetectedAt)
	if prevTime.IsZero() || newDetectedAt.Sub(prevTime) > constants.InversePairSuppressionWindow {
		return false
	}
	if len(prev.Changes) != len(newChanges) {
		return false
	}
	return changesAreInverse(prev.Changes, newChanges)
}

func changesAreInverse(prev, next []application.ApplicationChange) bool {
	expected := make(map[changeKey]int, len(prev))
	for _, c := range prev {
		expected[inverseOf(c)]++
	}
	for _, c := range next {
		k := signatureOf(c)
		if expected[k] <= constants.DefaultInitValue {
			return false
		}
		expected[k]--
	}
	for _, count := range expected {
		if count != constants.DefaultInitValue {
			return false
		}
	}
	return true
}
