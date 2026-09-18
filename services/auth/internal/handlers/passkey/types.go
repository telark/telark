package passkey

import "github.com/go-webauthn/webauthn/protocol"

type (
	RegisterStartRequest struct {
		Email       string `json:"email,omitempty"`
		EnrollToken string `json:"enrollToken,omitempty"`
	}
	EnrollLinkResponse struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expiresAt"`
	}
	RegisterStartResponse struct {
		Options *protocol.CredentialCreation `json:"options"`
	}
	UpdatePasskeyRequest struct {
		DeviceName        *string `json:"deviceName,omitempty"`
		LastUsedTimestamp *string `json:"lastUsedTimestamp,omitempty"`
	}
	DeletePasskeyRequest struct {
		ForceLastDelete bool `json:"forceLastDelete,omitempty"`
		CleanupOrphaned bool `json:"cleanupOrphaned,omitempty"`
	}
)
