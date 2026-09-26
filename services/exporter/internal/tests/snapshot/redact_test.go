package snapshot

import (
	"testing"

	"github.com/telark/exporter/internal/constants"
	snaputil "github.com/telark/exporter/internal/utils/snapshot"
)

const (
	fieldData       = "data"
	fieldStringData = "stringData"
	keyToken        = "token"
	keyPassword     = "password"
	dummyEncoded    = "ZHVtbXk="
	dummyPlain      = "dummy"
)

func TestRedactSecretsMasksValuesKeepsKeys(t *testing.T) {
	items := []map[string]any{
		{
			constants.FieldKind: constants.KindSecret,
			fieldData:           map[string]any{keyToken: dummyEncoded},
			fieldStringData:     map[string]any{keyPassword: dummyPlain},
		},
		{
			constants.FieldKind: constants.KindConfigMap,
			fieldData:           map[string]any{keyToken: dummyPlain},
		},
		{constants.FieldKind: constants.KindSecret},
	}

	snaputil.RedactSecrets(items)

	secret := items[constants.DefaultInitValue]
	if got := mapAt(t, secret, fieldData)[keyToken]; got != constants.SecretValueRedacted {
		t.Errorf("data.%s = %v, want the marker", keyToken, got)
	}
	if got := mapAt(t, secret, fieldStringData)[keyPassword]; got != constants.SecretValueRedacted {
		t.Errorf("stringData.%s = %v, want the marker", keyPassword, got)
	}
	if got := mapAt(t, items[constants.DefaultIncrementValue], fieldData)[keyToken]; got != dummyPlain {
		t.Errorf("ConfigMap data was touched: %v", got)
	}
}
