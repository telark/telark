package auth

type AuthChallenge struct {
	Challenge        string `json:"challenge"`
	UserID           string `json:"userId"`
	CreatedTimestamp string `json:"createdTimestamp"`
	ExpiresTimestamp string `json:"expiresTimestamp"`
}

type UserPasskey struct {
	UserID            string  `json:"userId"`
	CredentialID      string  `json:"credentialId"`
	PublicKey         string  `json:"publicKey"`
	DeviceName        string  `json:"deviceName"`
	DeviceType        string  `json:"deviceType"` // "platform" or "cross-platform"
	CreationTimestamp *string `json:"creationTimestamp,omitempty"`
	LastUsedTimestamp *string `json:"lastUsedTimestamp,omitempty"`
}

type UserSession struct {
	UserID           string  `json:"userId"`
	SessionToken     string  `json:"sessionToken"`
	CreatedTimestamp string  `json:"createdTimestamp"`
	ExpiresTimestamp string  `json:"expiresTimestamp"`
	IPAddress        *string `json:"ipAddress,omitempty"`
}
