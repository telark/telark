package user

type UserAsResource struct {
	ID           string     `json:"id"`
	Username     string     `json:"username"`
	Fullname     string     `json:"fullname"`
	Email        string     `json:"email"`
	RoleID       *string    `json:"roleID,omitempty"`
	GroupID      *string    `json:"groupID,omitempty"`
	CreationDate string     `json:"creationDate"`
	Avatar       *Avatar    `json:"avatar,omitempty"`
	Status       UserStatus `json:"status"`
}

type Avatar struct {
	Style string `json:"style"`
	Seed  string `json:"seed"`
}

type UserStatus struct {
	Phase       string  `json:"phase"`
	LastLoginAt *string `json:"lastLoginAt,omitempty"`
}
