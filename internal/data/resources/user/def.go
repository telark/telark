package user

type User struct {
	ID             string          `json:"id"`
	Username       string          `json:"username"`
	Fullname       string          `json:"fullname"`
	Email          string          `json:"email"`
	RoleRefs       []*string       `json:"roleRefs,omitempty"`
	GroupRefs      []*string       `json:"groupRefs,omitempty"`
	Identities     []*UserIdentity `json:"identities,omitempty"`
	CreationDate   string          `json:"creationDate"`
	LastUpdateDate *string         `json:"lastUpdateDate,omitempty"`
	Avatar         *Avatar         `json:"avatar,omitempty"`
	Settings       *UserSettings   `json:"settings,omitempty"`
	Status         UserStatus      `json:"status"`
	// Chart-managed bootstrap administrator; written by services only.
	Bootstrap bool `json:"bootstrap,omitempty"`
	// Projected from metadata by the owner while the cleanup finalizer holds the record.
	DeletionTimestamp *string `json:"deletionTimestamp,omitempty"`
}

type UserSettings struct {
	Timezone string `json:"timezone,omitempty"`
	Region   string `json:"region,omitempty"`
	Theme    string `json:"theme,omitempty"`
}

type UserIdentity struct {
	Provider string `json:"provider,omitempty"`
	Issuer   string `json:"issuer,omitempty"`
	Subject  string `json:"subject,omitempty"`
}

type Avatar struct {
	Style string `json:"style"`
	Seed  string `json:"seed"`
}

type UserStatus struct {
	Phase       string  `json:"phase"`
	LastLoginAt *string `json:"lastLoginAt,omitempty"`
}

type AccountPhase string

const (
	AccountPhaseActive    AccountPhase = "active"
	AccountPhaseInactive  AccountPhase = "inactive"
	AccountPhaseSuspended AccountPhase = "suspended"
)
