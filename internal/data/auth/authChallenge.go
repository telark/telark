package auth

type AuthChallenge struct {
	Challenge        string `json:"challenge"`
	UserID           string `json:"userId"`
	CreatedTimestamp string `json:"createdTimestamp"`
	ExpiresTimestamp string `json:"expiresTimestamp"`
}
