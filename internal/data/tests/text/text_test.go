package text

import (
	"regexp"
	"strings"
	"testing"

	"github.com/telark/telark/internal/data/text"
)

const (
	field      = "name"
	minRunes   = 3
	maxRunes   = 5
	maxItems   = 2
	itemMin    = 1
	itemMax    = 3
	atMax      = "abcde"
	atMin      = "abc"
	cutResult  = "abcd"
	accented   = "ééééé"
	itemA      = "a"
	itemB      = "b"
	errHidden  = "name contains a control or invisible character"
	zeroWidth  = "\u200b"
	bidiToggle = "\u202e"
	multiLine  = "ab\ncd"
	prefix     = "ab"
	itemC      = "c"
)

var namePattern = regexp.MustCompile(`^[a-z-]+$`)

func TestCheck(t *testing.T) {
	rule := text.Rule{Field: field, Min: minRunes, Max: maxRunes}
	optional := text.Rule{Field: field, Max: maxRunes}
	patterned := text.Rule{Field: field, Max: maxRunes, Pattern: namePattern}
	cases := []struct {
		name    string
		value   string
		rule    text.Rule
		want    string
		wantErr string
	}{
		{name: "under min", value: "ab", rule: rule, wantErr: "name must be at least 3 characters"},
		{name: "at min", value: atMin, rule: rule, want: atMin},
		{name: "at max", value: atMax, rule: rule, want: atMax},
		{name: "over max", value: "abcdef", rule: rule, wantErr: "name must be at most 5 characters"},
		{name: "runes not bytes", value: accented, rule: rule, want: accented},
		{name: "trimmed before counting", value: "  abcde \n", rule: rule, want: atMax},
		{name: "empty required", value: "   ", rule: rule, wantErr: "name is required"},
		{name: "empty optional", value: "", rule: optional, want: ""},
		{name: "control char", value: "ab\x07c", rule: rule, wantErr: errHidden},
		{name: "newline single line", value: multiLine, rule: rule, wantErr: errHidden},
		{name: "newline multiline", value: multiLine, rule: text.Rule{Field: field, Max: maxRunes, Multiline: true}, want: multiLine},
		{name: "zero width space", value: prefix + zeroWidth + itemC, rule: rule, wantErr: errHidden},
		{name: "bidi override", value: prefix + bidiToggle + itemC, rule: rule, wantErr: errHidden},
		{name: "invalid utf8", value: "ab\xffc", rule: rule, wantErr: "name is not valid UTF-8"},
		{name: "pattern match", value: "a-b", rule: patterned, want: "a-b"},
		{name: "pattern refused", value: "a b", rule: patterned, wantErr: "name contains characters that are not allowed"},
		{name: "pattern skips empty optional", value: "", rule: patterned, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := text.Check(tc.value, tc.rule)
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v, want %q", got, err, tc.want)
			}
		})
	}
}

func TestCheckList(t *testing.T) {
	rule := text.Rule{Field: field, Min: itemMin, Max: itemMax}
	cases := []struct {
		name    string
		values  []string
		want    []string
		wantErr string
	}{
		{name: "under max items", values: []string{" a "}, want: []string{itemA}},
		{name: "at max items", values: []string{itemA, itemB}, want: []string{itemA, itemB}},
		{name: "over max items", values: []string{itemA, itemB, itemC}, wantErr: "name holds at most 2 items"},
		{name: "bad item", values: []string{itemA, cutResult}, wantErr: "name must be at most 3 characters"},
		{name: "empty list", values: []string{}, want: []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := text.CheckList(tc.values, maxItems, rule)
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("got %v, %v, want %v", got, err, tc.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{name: "under limit", value: atMin, want: atMin},
		{name: "at limit", value: atMax, want: atMax},
		{name: "over limit", value: "abcdef", want: atMax},
		{name: "runes not bytes", value: accented + "é", want: accented},
		{name: "hidden chars dropped", value: "a" + zeroWidth + "b\x00c\nd" + bidiToggle, want: cutResult},
		{name: "invalid bytes dropped", value: "ab\xffc", want: atMin},
		{name: "trimmed after cut", value: "abcd efg", want: cutResult},
		{name: "trimmed before cut", value: "   abcdef", want: atMax},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := text.Truncate(tc.value, maxRunes); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
