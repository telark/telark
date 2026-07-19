package constants

const (
	PrefixNotifierService        = "NotifierService: "
	PrefixManagerSubscriber      = "ManagerSubscriber: "
	EmptyString                  = ""
	DefaultInitValue             = 0
	DefaultAdd                   = 1
	SubjectPartsMin              = 3
	SignalChanBuffer             = 1
	NatsFetchBatchSize           = 10
	DefaultMaxRetries            = 3
	AckWaitSeconds               = 5
	FetchMaxWaitSeconds          = 5
	RetryDelaySeconds            = 5
	ProcessTimeoutSeconds        = 30
	FieldNameKey                 = "name"
	ServiceID                    = "notifier"
	NatsInitRetryIntervalSeconds = 5
	NatsInitMaxWaitSeconds       = 300
	StatusServerPort             = ":8080"
	StatusServerReadTimeoutSec   = 5
	ConnectionLogIntervalSeconds = 30
)
