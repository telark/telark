package plans

import (
	"testing"

	"github.com/telark/data/plans"
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
