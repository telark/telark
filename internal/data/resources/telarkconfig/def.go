package telarkconfig

const (
	TelarkConfigResourceName = "default"
	// Key of the OIDC trust Secret that holds the Google JWK set.
	OIDCSecretKey = "googleJwkJson"
)

const (
	FieldExcludedNamespaces = "excludedNamespaces"
	FieldUserSettings       = "userSettings"
	FieldAI                 = "ai"
	FieldSnapshots          = "snapshots"
	FieldCluster            = "cluster"
	FieldOIDC               = "oidc"
	FieldAIModel            = "model"
	FieldAIAutoAnalyze      = "autoAnalyze"
)

type TelarkConfig struct {
	UserSettings       UserSettings    `json:"userSettings"`
	AI                 AIConfig        `json:"ai"`
	Cluster            Cluster         `json:"cluster"`
	ExcludedNamespaces []string        `json:"excludedNamespaces,omitempty"`
	Snapshots          SnapshotsConfig `json:"snapshots"`
	OIDC               OIDCConfig      `json:"oidc"`
}

type UserSettings struct {
	FetchIntervalSeconds int `json:"fetchIntervalSeconds"`
}

type AIConfig struct {
	Enabled     bool   `json:"enabled"`
	Model       string `json:"model"`
	AutoAnalyze bool   `json:"autoAnalyze"`
}

type Cluster struct {
	Version string `json:"version"`
}

type SnapshotsConfig struct {
	MaxPerApp int `json:"maxPerApp"`
}

// GoogleJWKJSON is the trust anchor for identity tokens when EgressAllowed is false. View only:
// it lives in the OIDC trust Secret under OIDCSecretKey and is never persisted in the CR.
type OIDCConfig struct {
	Enabled        bool   `json:"enabled"`
	GoogleClientID string `json:"googleClientID"`
	EgressAllowed  bool   `json:"egressAllowed"`
	GoogleJWKJSON  string `json:"googleJwkJson,omitempty"`
}
