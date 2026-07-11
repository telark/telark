package passkey

import "github.com/go-webauthn/webauthn/protocol"

type (
	RegisterStartRequest struct {
		Email string `json:"email,omitempty"`
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
