package rate_limiting

import "time"

const (
	METRICS_API_RATE_LIMIT = 1 * time.Second
	K8S_API_RATE_LIMIT     = 500 * time.Millisecond
)
