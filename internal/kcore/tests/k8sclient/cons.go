package k8sclient

const (
	TestQPSVar   = "TEST_RL_QPS"
	TestBurstVar = "TEST_RL_BURST"

	TestDefaultQPS   = 7
	TestDefaultBurst = 14
	TestEnvQPS       = 30
	TestEnvBurst     = 60

	ExpectedRateLimitFromEnv = "(%q,%q) = (%v,%d), want (%v,%d)"
)
