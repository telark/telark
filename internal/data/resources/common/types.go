package common

type (
	Type       string
	SourceType string
	Mode       string
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
	GROUPER             Type       = "Grouper"
	APP_WORKLOAD        Type       = "AppWorkload"
	BATCH_WORKLOAD      Type       = "BatchWorkload"
	BRIDGE              Type       = "Bridge"
	NS                  SourceType = "Namespace"
	DEPLOY              SourceType = "Deployment"
	STATEFUL_SET        SourceType = "StatefulSet"
	DAEMON_SET          SourceType = "DaemonSet"
	SVC                 SourceType = "Service"
	JOB                 SourceType = "Job"
	CRON_JOB            SourceType = "CronJob"
	SYNC_MODE_MANUAL    Mode       = "manual"
	SYNC_MODE_AUTO      Mode       = "auto"
	SYNC_STATUS_SUCCESS SyncStatus = "success"
	SYNC_STATUS_FAILED  SyncStatus = "failed"
)
