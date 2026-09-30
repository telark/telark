package resources

import (
	"reflect"
	"strings"
	"testing"

	"github.com/telark/telark/services/exporter/internal/constants"
	applicationutil "github.com/telark/telark/services/exporter/internal/utils/resources/application"
)

const (
	appDisplayName = "Shop"
	appDescription = "storefront"
)

// A 5000-character displayName was stored live; the handler now answers 400
// with the limit the CRD enforces.
func TestValidateEditableFields(t *testing.T) {
	longName := strings.Repeat(valueX, constants.MaxApplicationDisplayNameLength+constants.DefaultIncrementValue)
	longDescription := strings.Repeat(valueX, constants.MaxApplicationDescriptionLength+constants.DefaultIncrementValue)
	tests := []struct {
		name    string
		target  map[string]any
		wantErr bool
	}{
		{"within limits", map[string]any{constants.FieldDisplayName: appDisplayName, constants.FieldDescription: appDescription}, false},
		{"display name at limit", map[string]any{constants.FieldDisplayName: strings.Repeat("é", constants.MaxApplicationDisplayNameLength)}, false},
		{"display name too long", map[string]any{constants.FieldDisplayName: longName}, true},
		{"description too long", map[string]any{constants.FieldDescription: longDescription}, true},
		{"non-string display name", map[string]any{constants.FieldDisplayName: wantReplacedIDs}, true},
		{"non-string description", map[string]any{constants.FieldDescription: []any{valueX}}, true},
		{"null clears the field", map[string]any{constants.FieldDescription: nil}, false},
		{"fields absent", map[string]any{constants.FieldHistory: map[string]any{}}, false},
	}
	for _, tt := range tests {
		if err := applicationutil.ValidateEditableFields(tt.target); (err != nil) != tt.wantErr {
			t.Errorf("%s: err = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

// A top-level displayName was merged into the object root and pruned by the
// CRD: the patch answered 200 and the value was lost.
func TestLiftEditableFields(t *testing.T) {
	specKey, nameKey, descKey := constants.SpecField, constants.FieldDisplayName, constants.FieldDescription
	tests := []struct {
		name       string
		patch      map[string]any
		wantPatch  map[string]any
		wantLifted bool
	}{
		{
			"top level only",
			map[string]any{nameKey: appDisplayName, descKey: appDescription},
			map[string]any{specKey: map[string]any{nameKey: appDisplayName, descKey: appDescription}},
			true,
		},
		{
			"merged into existing spec",
			map[string]any{descKey: appDescription, specKey: map[string]any{nameKey: appDisplayName}},
			map[string]any{specKey: map[string]any{nameKey: appDisplayName, descKey: appDescription}},
			true,
		},
		{
			"spec wrapper untouched",
			map[string]any{specKey: map[string]any{nameKey: appDisplayName}},
			map[string]any{specKey: map[string]any{nameKey: appDisplayName}},
			false,
		},
		{
			"non-object spec untouched",
			map[string]any{nameKey: appDisplayName, specKey: nil},
			map[string]any{nameKey: appDisplayName, specKey: nil},
			false,
		},
	}
	for _, tt := range tests {
		spec, lifted := applicationutil.LiftEditableFields(tt.patch)
		if lifted != tt.wantLifted || !reflect.DeepEqual(tt.patch, tt.wantPatch) {
			t.Errorf("%s: patch = %v lifted %v, want %v lifted %v", tt.name, tt.patch, lifted, tt.wantPatch, tt.wantLifted)
		}
		if lifted && !reflect.DeepEqual(spec, tt.wantPatch[constants.SpecField]) {
			t.Errorf("%s: spec = %v, want %v", tt.name, spec, tt.wantPatch[constants.SpecField])
		}
	}
}
