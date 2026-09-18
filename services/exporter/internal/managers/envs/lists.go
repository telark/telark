package envs

import (
	"strconv"

	"github.com/telark/exporter/internal/constants"
)

var listRenderConcurrency = constants.DefaultListRenderConcurrency

// A malformed value or one below the minimum falls back to the default.
func InitListRenderConcurrency() int {
	envVal := getEnv(constants.ListRenderConcurrencyEnv)
	if envVal == constants.EmptyString {
		listRenderConcurrency = constants.DefaultListRenderConcurrency
		return listRenderConcurrency
	}
	n, err := strconv.Atoi(envVal)
	if err != nil || n < constants.MinListRenderConcurrency {
		listRenderConcurrency = constants.DefaultListRenderConcurrency
		return listRenderConcurrency
	}
	listRenderConcurrency = n
	return listRenderConcurrency
}

func GetListRenderConcurrency() int {
	return listRenderConcurrency
}
