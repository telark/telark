package common

type Fasid struct {
	Name           string     `json:"name"`
	SourceName     string     `json:"sourceName"`
	Grouper        *string    `json:"grouper,omitempty"`
	Type           Type       `json:"type"`
	SourceType     SourceType `json:"sourceType"`
	CreationTime   string     `json:"creationTime"`
	LastUpdateTime string     `json:"lastUpdateTime"`
}

type (
	Type       string
	SourceType string
	Mode       string
)

const (
	GROUPER        Type = "Grouper"
	APP_WORKLOAD   Type = "AppWorkload"
	BATCH_WORKLOAD Type = "BatchWorkload"
	BRIDGE         Type = "Bridge"
)

const (
	NS           SourceType = "Namespace"
	DEPLOY       SourceType = "Deployment"
	STATEFUL_SET SourceType = "StatefulSet"
	DAEMON_SET   SourceType = "DaemonSet"
	SVC          SourceType = "Service"
	JOB          SourceType = "Job"
	CRON_JOB     SourceType = "CronJob"
)

const (
	SYNC_MODE_MANUAL Mode = "manual"
	SYNC_MODE_AUTO   Mode = "auto"
)

type Record struct {
	Name         string `json:"name"`
	Status       string `json:"status"`
	CreationTime string `json:"creationTime"`
}

type Config struct {
	History     []Record     `json:"history"`
	Sync        Sync         `json:"sync"`
	Maintenance *Maintenance `json:"maintenance,omitempty"`
}

type Sync struct {
	Mode           Mode   `json:"mode"`
	LastUpdateTime string `json:"lastUpdateTime"`
}

type Maintenance struct {
	Status          string `json:"status"`
	Name            string `json:"name"`
	AttachedWebhook string `json:"attachedWebhook"`
}
