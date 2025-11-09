package auth

type UserSession struct {
	UserID           string  `json:"userId"`
	SessionToken     string  `json:"sessionToken"`
	CreatedTimestamp string  `json:"createdTimestamp"`
	ExpiresTimestamp string  `json:"expiresTimestamp"`
	IPAddress        *string `json:"ipAddress,omitempty"`
}
