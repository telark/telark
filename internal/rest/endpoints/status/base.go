package status

import "github.com/telark/telark/internal/rest/base"

const (
	HealthCheck    base.Endpoint = "status/health"
	ReadinessCheck base.Endpoint = "status/ready"
	LivenessCheck  base.Endpoint = "status/live"
)
