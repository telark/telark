package utils

import (
	"strconv"
	"strings"
	"time"

	"github.com/telark/discovery/internal/constants"
)

func FormatAppTime(t time.Time) string {
	if t.IsZero() {
		return constants.EmptyString
	}
	return t.UTC().Format(time.RFC3339)
}

func ResourceKey(kind, name string) string {
	return kind + constants.PathSeparator + name
}

func StrPtr(s string) *string {
	p := new(string)
	*p = s
	return p
}

func StrconvInt(n int) string {
	return strconv.Itoa(n)
}

func SliceToMap(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

func IntSliceToSet(xs []int) map[int]bool {
	m := make(map[int]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}

func ParseRFC3339OrNano(raw string) time.Time {
	if strings.TrimSpace(raw) == constants.EmptyString {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t
	}
	return time.Time{}
}
