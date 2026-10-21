package telarkconfig

import "time"

const (
	TelarkConfigResourceName = "default"
	// Key of the OIDC trust Secret that holds the Google JWK set.
	OIDCSecretKey = "googleJwkJson"
)

const (
	FieldExcludedNamespaces = "excludedNamespaces"
	FieldAI                 = "ai"
	FieldSnapshots          = "snapshots"
	FieldCluster            = "cluster"
	FieldOIDC               = "oidc"
	FieldSelfRegistration   = "selfRegistration"
	FieldAIModel            = "model"
	FieldAIAutoAnalyze      = "autoAnalyze"
	FieldBootstrap          = "bootstrap"
	FieldBreakGlassAlerts   = "breakGlassAlerts"
	FieldNotifyAdmins       = "notifyAdmins"
)

// 72 hours with no sign-in by the bootstrap account complete a break-glass request, and a completed one re-issues
// its token for 24 hours while no passkey was enrolled: never settings.
const (
	BootstrapRequestWait   = 72 * time.Hour
	BootstrapReissueWindow = 24 * time.Hour
)

const (
	BootstrapKindRecover    = "recover"
	BootstrapKindReplace    = "replace"
	BootstrapStatePending   = "pending"
	BootstrapStateApproved  = "approved"
	BootstrapStateCanceled  = "canceled"
	BootstrapStateCompleted = "completed"
	BootstrapReasonSignedIn = "signed-in"
	BootstrapReasonDenied   = "denied"
	BootstrapReasonExpired  = "expired"
)

type TelarkConfig struct {
	AI                 AIConfig               `json:"ai"`
	Cluster            Cluster                `json:"cluster"`
	ExcludedNamespaces []string               `json:"excludedNamespaces,omitempty"`
	Snapshots          SnapshotsConfig        `json:"snapshots"`
	OIDC               OIDCConfig             `json:"oidc"`
	SelfRegistration   SelfRegistrationConfig `json:"selfRegistration"`
	BreakGlassAlerts   BreakGlassAlertsConfig `json:"breakGlassAlerts"`
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

type SelfRegistrationConfig struct {
	Enabled bool `json:"enabled"`
}

type BreakGlassAlertsConfig struct {
	NotifyAdmins bool `json:"notifyAdmins"`
}

// Status only, outside TelarkConfig: the config PATCH refuses the key from every caller, and GET config never shows it.
type BootstrapStatus struct {
	UserID   string             `json:"userID,omitempty"`
	Email    string             `json:"email,omitempty"`
	PinnedAt string             `json:"pinnedAt,omitempty"`
	Requests []BootstrapRequest `json:"requests,omitempty"`
}

type BootstrapRequest struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	TargetEmail string `json:"targetEmail,omitempty"`
	State       string `json:"state"`
	Reason      string `json:"reason,omitempty"`
	OpenedAt    string `json:"openedAt"`
	ApprovedAt  string `json:"approvedAt,omitempty"`
	CanceledAt  string `json:"canceledAt,omitempty"`
	CompletedAt string `json:"completedAt,omitempty"`
}
