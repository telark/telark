package envs

import (
	"strings"

	"github.com/telark/telark/services/exporter/internal/constants"
)

var (
	reportsPath         = constants.DefaultReportsPath
	reportsPVCName      = constants.DefaultReportsPVCName
	reportsPVCNamespace = constants.DefaultReportsPVCNamespace
)

func InitReportsPath() string {
	envPath := strings.TrimSpace(getEnv(constants.ReportsPathEnv))
	if envPath != constants.EmptyString {
		reportsPath = envPath
	}
	if name := getEnv(constants.ReportsPVCNameEnv); name != constants.EmptyString {
		reportsPVCName = name
	}
	if namespace := getEnv(constants.ReportsPVCNamespaceEnv); namespace != constants.EmptyString {
		reportsPVCNamespace = namespace
	}
	return reportsPath
}

func GetReportsPath() string {
	return reportsPath
}

func GetReportsPVCName() string {
	return reportsPVCName
}

func GetReportsPVCNamespace() string {
	return reportsPVCNamespace
}
