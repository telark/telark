package common

var (
	ManagedFields = []string{
		"apiVersion",
		"kind",
		"metadata.generation",
		"metadata.managedFields",
		"metadata.resourceVersion",
		"metadata.uid",
	}
)

type Config struct {
	History []HistoryEvent `json:"history,omitempty"`
	Sync    Sync           `json:"sync"`
}

type Sync struct {
	Mode     string   `json:"mode"`
	Settings Settings `json:"settings,omitempty"`
}

type Settings struct {
	Period   int    `json:"period"`
	LastSync string `json:"lastSync"`
}

type Fasid struct {
	Source Source `json:"source"`
}

type Source struct {
	Name           string `json:"name"`
	Grouper        string `json:"grouper,omitempty"`
	Kind           string `json:"kind"`
	CreationTime   string `json:"creationTime"`
	LastUpdateTime string `json:"lastUpdateTime"`
}

type Unified struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type HistoryEvent struct {
	Event        string `json:"event"`
	Status       string `json:"status"`
	CreationTime string `json:"creationTime"`
}
