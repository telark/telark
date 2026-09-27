package constants

import "time"

const (
	LoggerPrefixAuthz = "Authz: "

	// Bounds the revocation window; errors are never cached.
	AuthzSessionCacheTTL = 30 * time.Second
	AuthzGrantsCacheTTL  = 30 * time.Second
)

const (
	InfServiceHealthy           = "service is: healthy"
	InfServiceNotReady          = "service is: not ready"
	ReadinessReasonRedis        = "redis unreachable"
	ReadinessReasonBootstrap    = "bootstrap pending"
	ReadinessReasonTelarkConfig = "telarkconfig not loaded"
	ReadinessReasonExporter     = "exporter circuit open"
	ReadinessReasonKubernetes   = "kubernetes api unreachable"
)
