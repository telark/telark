package globalconfig

type GlobalConfig struct {
	UserSettings       UserSettings    `json:"userSettings"`
	AI                 AIConfig        `json:"ai"`
	Cluster            Cluster         `json:"cluster"`
	ExcludedNamespaces []string        `json:"excludedNamespaces,omitempty"`
	Snapshots          SnapshotsConfig `json:"snapshots"`
	OIDC               OIDCConfig      `json:"oidc"`
}

type UserSettings struct {
	Theme                string `json:"theme"`
	Density              string `json:"density"`
	UIViewSize           string `json:"uiViewSize"`
	FetchIntervalSeconds int    `json:"fetchIntervalSeconds"`
}

type AIConfig struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`
	APIKey   string `json:"apiKey"`
}

type Cluster struct {
	Version string `json:"version"`
}

type SnapshotsConfig struct {
	MaxPerApp int `json:"maxPerApp"`
}

type OIDCConfig struct {
	Enabled        bool   `json:"enabled"`
	GoogleClientID string `json:"googleClientID"`
}
