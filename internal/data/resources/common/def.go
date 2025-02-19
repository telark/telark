package common

type Fasid struct {
	Name           string     `json:"name"`
	SourceName     string     `json:"sourceName"`
	Grouper        string     `json:"grouper"`
	Type           Type       `json:"type"`
	SourceType     SourceType `json:"sourceType"`
	CreationTime   string     `json:"creationTime"`
	LastUpdateTime string     `json:"lastUpdateTime"`
}

type Type string
type SourceType string

const (
	GROUPER        Type = "Grouper"
	APP_WORKLOAD   Type = "AppWorkload"
	BATCH_WORKLOAD Type = "BatchWorkload"
	BRIDGE         Type = "Bridge"
)

const (
	NAMESPACE    SourceType = "Namespace"
	DEPLOYMENT   SourceType = "Deployment"
	STATEFUL_SET SourceType = "StatefulSet"
	DAEMON_SET   SourceType = "DaemonSet"
	SERVICE      SourceType = "Service"
	JOB          SourceType = "Job"
	CRON_JOB     SourceType = "CronJob"
)

type Config struct {
	History []Record `json:"history"`
	Sync    Sync     `json:"sync"`
}

type Sync struct {
	Mode           string `json:"mode"`
	LastUpdateTime string `json:"lastUpdateTime"`
}

type Record struct {
	Name         string `json:"name"`
	Status       string `json:"status"`
	CreationTime string `json:"creationTime"`
}
