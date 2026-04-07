package globalconfig

type GlobalConfig struct {
	UserSettings UserSettings `json:"userSettings"`
	AI           AIConfig     `json:"ai"`
	Cluster      Cluster      `json:"cluster"`
}

type UserSettings struct {
	Theme      string `json:"theme"`
	Density    string `json:"density"`
	UIViewSize string `json:"uiViewSize"`
}

type AIConfig struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`
	APIKey   string `json:"apiKey"`
}

type Cluster struct {
	Version string `json:"version"`
}
