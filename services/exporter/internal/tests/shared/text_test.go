package shared

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/telark/telark/services/exporter/internal/constants"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

const (
	fillChar       = "a"
	padding        = "  "
	clientIDSuffix = ".apps.googleusercontent.com"
	validStyle     = "pixel-art"
	unknownStyle   = "pixel-arts"
	zeroWidthSpace = 0x200b
)

var fieldLists = map[string][]sharedutils.TextField{
	"user":        sharedutils.UserTextFields,
	"group":       sharedutils.GroupTextFields,
	"role":        sharedutils.RoleTextFields,
	"category":    sharedutils.CategoryTextFields,
	"application": sharedutils.ApplicationTextFields,
	"plan":        sharedutils.PlanTextFields,
	"passkey":     sharedutils.PasskeyTextFields,
	"config":      sharedutils.ConfigTextFields,
}

// A value of n runes that matches the field's character set.
func fill(field string, n int) string {
	if field == constants.FieldOIDCGoogleClientID {
		return strings.Repeat(fillChar, n-len(clientIDSuffix)) + clientIDSuffix
	}
	return strings.Repeat(fillChar, n)
}

// The body a client sends for a dotted field path.
func bodyWith(path string, value any) map[string]any {
	keys := strings.Split(path, constants.BodyFieldPathSep)
	body := map[string]any{keys[len(keys)-constants.IndexLastElementOffset]: value}
	for i := len(keys) - constants.IndexLastElementOffset - constants.DefaultIncrementValue; i >= constants.DefaultInitValue; i-- {
		body = map[string]any{keys[i]: body}
	}
	return body
}

func leaf(body map[string]any, path string) any {
	keys := strings.Split(path, constants.BodyFieldPathSep)
	for _, key := range keys[:len(keys)-constants.IndexLastElementOffset] {
		next, isMap := body[key].(map[string]any)
		if !isMap {
			return nil
		}
		body = next
	}
	return body[keys[len(keys)-constants.IndexLastElementOffset]]
}

func items(n int, value string) []any {
	out := make([]any, n)
	for i := range out {
		out[i] = value
	}
	return out
}

type limitCase struct {
	name  string
	value any
	ok    bool
}

func limitCases(field sharedutils.TextField) []limitCase {
	if field.Field == constants.FieldAvatarStyle {
		return []limitCase{{"known style", validStyle, true}, {"unknown style", unknownStyle, false}}
	}
	at, over := fill(field.Field, field.Max), fill(field.Field, field.Max+constants.DefaultIncrementValue)
	cases := []limitCase{{"under", fill(field.Field, field.Max-constants.DefaultIncrementValue), true}, {"at", at, true}, {"over", over, false}}
	if field.Min > constants.DefaultInitValue {
		cases = append(cases,
			limitCase{"at min", fill(field.Field, field.Min), true},
			limitCase{"under min", fill(field.Field, field.Min-constants.DefaultIncrementValue), false})
	}
	if field.MaxItems == constants.DefaultInitValue {
		return cases
	}
	for i := range cases {
		cases[i].value = items(constants.DefaultIncrementValue, cases[i].value.(string))
	}
	return append(cases,
		limitCase{"items at max", items(field.MaxItems, at), true},
		limitCase{"items over max", items(field.MaxItems+constants.DefaultIncrementValue, at), false})
}

// Every field is checked at, under and over its limits; a refusal names the field.
func TestCheckTextFieldsLimits(t *testing.T) {
	for list, fields := range fieldLists {
		for _, field := range fields {
			for _, c := range limitCases(field) {
				err := sharedutils.CheckTextFields(bodyWith(field.Field, c.value), fields...)
				if (err == nil) != c.ok {
					t.Errorf("%s %s %s: err = %v, want ok %v", list, field.Field, c.name, err, c.ok)
				}
				if err != nil && !strings.Contains(err.Error(), field.Field) {
					t.Errorf("%s %s %s: %q does not name the field", list, field.Field, c.name, err)
				}
			}
		}
	}
}

// A stored value is the trimmed one, in a nested object and in a list too.
func TestCheckTextFieldsTrimsInPlace(t *testing.T) {
	cases := []struct {
		fields []sharedutils.TextField
		path   string
		value  any
		want   any
	}{
		{sharedutils.UserTextFields, constants.FieldFullname, padding + fillChar + padding, fillChar},
		{sharedutils.UserTextFields, constants.FieldAvatarSeed, padding + fillChar, fillChar},
		{sharedutils.PlanTextFields, constants.FieldPlanScopeNs, []any{padding + fillChar}, []any{fillChar}},
	}
	for _, c := range cases {
		body := bodyWith(c.path, c.value)
		if err := sharedutils.CheckTextFields(body, c.fields...); err != nil {
			t.Fatalf("%s: %v", c.path, err)
		}
		if got := leaf(body, c.path); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s = %#v, want %#v", c.path, got, c.want)
		}
	}
}

// Absent fields, null, other types and lists of other types are the schema's to refuse.
func TestCheckTextFieldsLeavesOtherShapes(t *testing.T) {
	bodies := []map[string]any{
		{},
		{constants.FieldFullname: nil},
		{constants.FieldFullname: constants.DefaultIncrementValue},
		{constants.FieldAvatar: fillChar},
		{constants.FieldScope: map[string]any{constants.FieldScopeNamespaces: fillChar}},
		{constants.FieldScope: map[string]any{constants.FieldScopeNamespaces: []any{constants.DefaultIncrementValue}}},
	}
	for _, body := range bodies {
		if err := sharedutils.CheckTextFields(body, append(sharedutils.UserTextFields, sharedutils.PlanTextFields...)...); err != nil {
			t.Errorf("%v: %v", body, err)
		}
	}
}

// Characters outside a field's set are refused, as are invisible ones anywhere.
func TestCheckTextFieldsCharacters(t *testing.T) {
	zeroWidth := string(rune(zeroWidthSpace))
	cases := []struct {
		fields []sharedutils.TextField
		path   string
		value  string
		ok     bool
	}{
		{sharedutils.UserTextFields, constants.FieldUsername, "release lead", false},
		{sharedutils.UserTextFields, constants.FieldFullname, "Zoë O'Neil-Ng Jr.", true},
		{sharedutils.UserTextFields, constants.FieldFullname, "release_lead", false},
		{sharedutils.UserTextFields, constants.FieldFullname, fillChar + zeroWidth + fillChar, false},
		{sharedutils.GroupTextFields, constants.FieldDescription, fillChar + "\n" + fillChar, false},
		{sharedutils.RoleTextFields, constants.FieldDescription, fillChar + "\n" + fillChar, true},
		{sharedutils.PlanTextFields, constants.FieldName, "freeze (2)", true},
		{sharedutils.ConfigTextFields, constants.FieldOIDCGoogleClientID, "client id" + clientIDSuffix, false},
		{sharedutils.ConfigTextFields, constants.FieldOIDCGoogleClientID, fillChar, false},
	}
	for _, c := range cases {
		if err := sharedutils.CheckTextFields(bodyWith(c.path, c.value), c.fields...); (err == nil) != c.ok {
			t.Errorf("%s %q: err = %v, want ok %v", c.path, c.value, err, c.ok)
		}
	}
}

// A number is checked at, under and over both ends of its range; other shapes pass through.
func TestCheckRangeOrRespond(t *testing.T) {
	const minValue, maxValue = constants.MinRoleValidityHours, constants.MaxRoleValidityHours
	cases := []struct {
		value any
		ok    bool
	}{
		{float64(minValue - constants.DefaultIncrementValue), false},
		{float64(minValue), true},
		{float64(minValue + constants.DefaultIncrementValue), true},
		{float64(maxValue - constants.DefaultIncrementValue), true},
		{float64(maxValue), true},
		{float64(maxValue + constants.DefaultIncrementValue), false},
		{fillChar, true},
		{nil, true},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		ok := sharedutils.CheckRangeOrRespond(w, bodyWith(constants.FieldValidityHours, c.value), constants.FieldValidityHours, minValue, maxValue)
		if ok != c.ok || (!ok && (w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), constants.FieldValidityHours))) {
			t.Errorf("%v: ok = %v (%d %s), want %v", c.value, ok, w.Code, w.Body.String(), c.ok)
		}
	}
	if !sharedutils.CheckRangeOrRespond(httptest.NewRecorder(), map[string]any{}, constants.FieldValidityHours, minValue, maxValue) {
		t.Error("an absent field was refused")
	}
}
