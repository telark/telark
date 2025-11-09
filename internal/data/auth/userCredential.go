package auth

type UserCredential struct {
	UserID            string  `json:"userId"`
	CredentialID      string  `json:"credentialId"`
	PublicKey         string  `json:"publicKey"`
	DeviceName        string  `json:"deviceName"`
	DeviceType        string  `json:"deviceType"` // "platform" or "cross-platform"
	CreationTimestamp *string `json:"creationTimestamp,omitempty"`
	LastUsedTimestamp *string `json:"lastUsedTimestamp,omitempty"`
}
