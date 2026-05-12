package constants

import "time"

const (
	K8sTransientRetrySteps           = 3
	K8sTransientRetryInitial         = 200 * time.Millisecond
	K8sTransientRetryFactor          = 2.0
	K8sTransientRetryJitter          = 0.2
	K8sTransientRetryCap             = 1 * time.Second
	K8sApplyRetrySteps               = 3
	K8sApplyRetryInitial             = 250 * time.Millisecond
	K8sApplyRetryFactor              = 2.0
	K8sApplyRetryJitter              = 0.2
	K8sApplyRetryCap                 = 1 * time.Second
	K8sClientRateLimitWaitSub        = "client rate limiter Wait"
	K8sReachabilityProbeTimeout      = 1 * time.Second
	K8sReachabilityCacheTTL          = 5 * time.Second
	K8sReachabilityProbeChannelDepth = 1
	ZeroJitterFraction               = 0.0
	JitterCenterOffset               = 1.0
	JitterRangeMultiplier            = 2.0
)
