package shared

import "github.com/telark/telark/services/discovery/internal/constants"

func CoalesceStrings(s []string) []string {
	if len(s) > constants.DefaultInitValue {
		return s
	}
	return []string{}
}
