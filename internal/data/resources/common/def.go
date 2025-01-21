package common

type Fasid struct {
	Name           string `json:"name"`
	Grouper        string `json:"grouper,omitempty"`
	Type           string `json:"type"`
	OriginalType   string `json:"originalType"`
	CreationTime   string `json:"creationTime"`
	LastUpdateTime string `json:"lastUpdateTime"`
}

type Config struct {
	History []HistoryEvent `json:"history,omitempty"`
	Sync    Sync           `json:"sync"`
}

type Sync struct {
	Mode           string `json:"mode"`
	LastUpdateTime string `json:"lastUpdateTime"`
}

type HistoryEvent struct {
	Name         string `json:"event"`
	Status       string `json:"status"`
	CreationTime string `json:"creationTime"`
}
