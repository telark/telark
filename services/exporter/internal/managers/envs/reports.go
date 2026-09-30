package envs

import (
	"strings"

	"github.com/telark/telark/services/exporter/internal/constants"
)

var reportsPath = constants.DefaultReportsPath

func InitReportsPath() string {
	envPath := strings.TrimSpace(getEnv(constants.ReportsPathEnv))
	if envPath != constants.EmptyString {
		reportsPath = envPath
	}
	return reportsPath
}

func GetReportsPath() string {
	return reportsPath
}
