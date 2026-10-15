package plans

import (
	"testing"

	"github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/data/plans"
)

const (
	tplImageTags   = "block-image-tags"
	tplImageTypes  = "block-image-types"
	tplBlockUpdate = "block-update"
	keyTags        = "tags"
	keyPatterns    = "imagePatterns"
	keyUnknown     = "bogus"
)

// Blank, whitespace and malformed entries used to pass and render a rule that matched nothing.
func TestValidateParamsRejectsInvalidEntries(t *testing.T) {
	cases := []struct {
		name     string
		template string
		params   map[string]any
		ok       bool
	}{
		{"tags valid", tplImageTags, map[string]any{keyTags: []any{"latest", "v1.2.3", "dev_1"}}, true},
		{"tags blank", tplImageTags, map[string]any{keyTags: []any{"", " "}}, false},
		{"tags malformed", tplImageTags, map[string]any{keyTags: []any{"bad tag!"}}, false},
		{"tags colon", tplImageTags, map[string]any{keyTags: []any{"UPPER:x"}}, false},
		{"tags leading dot", tplImageTags, map[string]any{keyTags: []any{".hidden"}}, false},
		{"patterns valid", tplImageTypes, map[string]any{keyPatterns: []string{"*busybox*", "*/untrusted/*"}}, true},
		{"patterns blank", tplImageTypes, map[string]any{keyPatterns: []string{""}}, false},
		{"patterns whitespace", tplImageTypes, map[string]any{keyPatterns: []string{"a b"}}, false},
		{"patterns empty", tplImageTypes, map[string]any{keyPatterns: []string{}}, false},
		{"unknown key", tplImageTags, map[string]any{keyTags: []any{"latest"}, keyUnknown: "x"}, false},
		{"unknown key on parameterless template", tplBlockUpdate, map[string]any{keyUnknown: "x"}, false},
		{"no params on parameterless template", tplBlockUpdate, nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tpl, ok := plans.GetTemplate(c.template)
			if !ok {
				t.Fatalf("template %s missing", c.template)
			}
			err := plans.ValidateParams(tpl, c.params)
			if (err == nil) != c.ok {
				t.Fatalf("ValidateParams(%s) = %v, want ok=%v", c.name, err, c.ok)
			}
		})
	}
}

const (
	keyOptional = "optional"
	keyFlag     = "flag"
	keyFree     = "freeform"
	typeBoolean = "boolean"
	anyEntry    = "any entry"
	entryNumber = 7
)

// Every pattern the catalog names is enforced: an entry of whitespace passes none of them.
func TestCatalogPatternsAreEnforced(t *testing.T) {
	for i := range plans.Templates {
		tpl := &plans.Templates[i]
		for _, spec := range tpl.Params {
			if spec.Pattern == constants.EmptyString {
				continue
			}
			if err := plans.ValidateParams(tpl, map[string]any{spec.Key: []string{" "}}); err == nil {
				t.Errorf("%s.%s accepted an entry its pattern %q refuses", tpl.ID, spec.Key, spec.Pattern)
			}
		}
	}
}

// Optional params and params of another type are not checked as arrays, a required array must
// be a list of strings, and a pattern the validator doesn't compile checks nothing (so every
// catalog pattern must be one it compiles, as the test above makes sure).
func TestValidateParamsSpecRules(t *testing.T) {
	tpl := &plans.Template{ID: keyUnknown, Params: []plans.ParamSpec{
		{Key: keyOptional, Type: plans.ParamTypeStringArray},
		{Key: keyFlag, Type: typeBoolean, Required: true},
		{Key: keyFree, Type: plans.ParamTypeStringArray, Required: true, Pattern: `^unknown$`},
	}}
	cases := []struct {
		name   string
		params map[string]any
		ok     bool
	}{
		{"optional left out, unknown pattern", map[string]any{keyFlag: true, keyFree: []string{anyEntry}}, true},
		{"required left out", map[string]any{keyFlag: true}, false},
		{"required null", map[string]any{keyFlag: true, keyFree: nil}, false},
		{"entry of another type", map[string]any{keyFlag: true, keyFree: []any{anyEntry, entryNumber}}, false},
		{"not a list", map[string]any{keyFlag: true, keyFree: anyEntry}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := plans.ValidateParams(tpl, c.params); (err == nil) != c.ok {
				t.Fatalf("ValidateParams(%s) = %v, want ok=%v", c.name, err, c.ok)
			}
		})
	}
}
