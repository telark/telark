package constants

const (
	PrefixNotifierService        = "NotifierService: "
	PrefixManagerSubscriber      = "ManagerSubscriber: "
	EmptyString                  = ""
	DefaultInitValue             = 0
	DefaultAdd                   = 1
	SubjectPartsMin              = 3
	NatsFetchBatchSize           = 64
	DefaultMaxRetries            = 3
	AckWaitSeconds               = 5
	FetchMaxWaitSeconds          = 5
	RetryDelaySeconds            = 5
	ProcessTimeoutSeconds        = 30
	FieldNameKey                 = "name"
	ServiceID                    = "notifier"
	NatsInitRetryIntervalSeconds = 5
	NatsInitMaxWaitSeconds       = 300
	NatsStartRetrySeconds        = 15
	StatusServerPort             = ":8080"
	StatusServerReadTimeoutSec   = 5
	ConnectionLogIntervalSeconds = 30
	ApplyWorkerCount             = 8
	ApplyWorkerQueueSize         = 32
	DrainTimeoutSeconds          = 20
	EnvApplyWorkers              = "NOTIFIER_APPLY_WORKERS"
)
