package user

type UserAsResource struct {
	ID               string     `json:"id"`
	Username         string     `json:"username"`
	Fullname         string     `json:"fullname"`
	Email            string     `json:"email"`
	AssignedRolesIDs []string   `json:"assignedRolesIDs"`
	AssignedGroupIDs []string   `json:"assignedGroupIDs"`
	CreationDate     string     `json:"creationDate"`
	LastUpdateDate   *string    `json:"lastUpdateDate,omitempty"`
	Avatar           *Avatar    `json:"avatar,omitempty"`
	Status           UserStatus `json:"status"`
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
