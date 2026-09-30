package diff

import (
	"github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/services/discovery/internal/core/applications/history/changes"
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
			value: derefChangeValue(c.OldValue) + fieldValueSep + derefChangeValue(c.NewValue),
		}
	default:
		return changeKey{field: c.Field, typ: c.ChangeType}
	}
}
