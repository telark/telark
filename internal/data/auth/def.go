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
	BackupEligible    bool    `json:"backupEligible"` // Backup Eligible flag from WebAuthn authenticator data
	BackupState       bool    `json:"backupState"`    // Backup State flag from WebAuthn authenticator data
}

type UserSession struct {
	UserID           string         `json:"userId"`
	SessionToken     string         `json:"sessionToken"`
	CreatedTimestamp string         `json:"createdTimestamp"`
	ExpiresTimestamp string         `json:"expiresTimestamp"`
	IPAddress        *string        `json:"ipAddress,omitempty"`
	DeviceMetadata   DeviceMetadata `json:"deviceMetadata"`
}

type DeviceMetadata struct {
	Browser   *string `json:"browser,omitempty"`
	Device    *string `json:"device,omitempty"`
	OS        *string `json:"os,omitempty"`
	Location  *string `json:"location,omitempty"`
	UserAgent *string `json:"userAgent,omitempty"`
}
