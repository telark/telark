package shared

import "github.com/telark/discovery/internal/constants"

func DefaultSlice[T any](s, empty []T) []T {
	if len(s) > constants.DefaultInitValue {
		return s
	}
	return empty
}

func CoalesceStrings(s []string) []string {
	if len(s) > constants.DefaultInitValue {
		return s
	}
	return []string{}
}
