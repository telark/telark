package shared

type (
	Type       string
	DataScope  string
	SyncStatus string
)

const (
	Application          Type       = "Application"
	SyncStatusSuccess    SyncStatus = "success"
	SyncStatusFailed     SyncStatus = "failed"
	FullDataScope        DataScope  = "full"
	ApplicationSpecScope DataScope  = "applicationSpec"
	NoneDataScope        DataScope  = "none"
)
