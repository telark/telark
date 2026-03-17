package shared

type (
	Type       string
	SourceType string
	Mode       string
	DataScope  string
	SyncStatus string
	Fasid      struct {
		Name         string     `json:"name"`
		SourceName   string     `json:"sourceName"`
		Grouper      string     `json:"grouper"`
		Type         Type       `json:"type"`
		SourceType   SourceType `json:"sourceType"`
		CreationTime string     `json:"creationTime"`
	}

	Record struct {
		Name         string `json:"name"`
		Status       string `json:"status"`
		CreationTime string `json:"creationTime"`
	}

	Config struct {
		History     []Record     `json:"history"`
		Sync        Sync         `json:"sync"`
		Maintenance *Maintenance `json:"maintenance,omitempty"`
	}

	Sync struct {
		Mode           Mode       `json:"mode"`
		LastStatus     SyncStatus `json:"lastStatus"`
		LastUpdateTime string     `json:"lastUpdateTime"`
	}

	Maintenance struct {
		Status          string `json:"status"`
		Name            string `json:"name"`
		AttachedWebhook string `json:"attachedWebhook"`
	}
)

const (
	Grouper           Type       = "Grouper"
	Application       Type       = "Application"
	AppWorkload       Type       = "AppWorkload"
	BatchWorkload     Type       = "BatchWorkload"
	Bridge            Type       = "Bridge"
	Ns                SourceType = "Namespace"
	Deploy            SourceType = "Deployment"
	StatefulSet       SourceType = "StatefulSet"
	DaemonSet         SourceType = "DaemonSet"
	Svc               SourceType = "Service"
	Job               SourceType = "Job"
	CronJob           SourceType = "CronJob"
	SyncModeManual    Mode       = "manual"
	SyncModeAuto      Mode       = "auto"
	SyncStatusSuccess SyncStatus = "success"
	SyncStatusFailed  SyncStatus = "failed"
	FasidDataScope    DataScope  = "fasid"
	CacidDataScope    	 DataScope  = "cacid"
	FullDataScope    	 DataScope  = "full"
	ApplicationSpecScope DataScope = "applicationSpec"
	NoneDataScope     	 DataScope  = "none"
)
