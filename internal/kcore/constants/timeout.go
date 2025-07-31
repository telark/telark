package constants

import "time"

const (
	METRICS_GET_TIMEOUT         = 10 * time.Second
	METRICS_LIST_TIMEOUT        = 15 * time.Second
	EVENT_FETCH_TIMEOUT         = 20 * time.Second
	POD_LIST_TIMEOUT            = 30 * time.Second
	POD_GET_TIMEOUT             = 15 * time.Second
	SERVICE_GET_TIMEOUT         = 20 * time.Second
	SERVICE_LIST_TIMEOUT        = 30 * time.Second
	NAMESPACE_LIST_TIMEOUT      = 30 * time.Second
	WORKLOAD_GET_TIMEOUT        = 30 * time.Second
	WORKLOAD_LIST_TIMEOUT       = 45 * time.Second
	ADMISSION_GET_TIMEOUT       = 15 * time.Second
	ADMISSION_PATCH_TIMEOUT     = 15 * time.Second
	ADMISSION_DELETE_TIMEOUT    = 15 * time.Second
	ADMISSION_CREATE_TIMEOUT    = 15 * time.Second
	DEFAULT_TIMEOUT             = 30 * time.Second
	SERVER_VERSION_TIMEOUT      = 30 * time.Second
	AVAILABILITY_CHECK_INTERVAL = 5 * time.Minute
)
