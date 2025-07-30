package timeout

import "time"

const (
	METRICS_GET_TIMEOUT  = 10 * time.Second
	METRICS_LIST_TIMEOUT = 15 * time.Second

	EVENT_FETCH_TIMEOUT = 20 * time.Second

	POD_LIST_TIMEOUT = 30 * time.Second
	POD_GET_TIMEOUT  = 15 * time.Second

	// Service operations timeouts
	SERVICE_GET_TIMEOUT  = 20 * time.Second
	SERVICE_LIST_TIMEOUT = 30 * time.Second

	// Namespace operations timeouts
	NAMESPACE_LIST_TIMEOUT = 30 * time.Second

	// Workload operations timeouts
	WORKLOAD_GET_TIMEOUT  = 30 * time.Second
	WORKLOAD_LIST_TIMEOUT = 45 * time.Second

	DEFAULT_TIMEOUT = 30 * time.Second
)
