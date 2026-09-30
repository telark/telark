package auth

import (
	"github.com/go-webauthn/webauthn/protocol"
	authdata "github.com/telark/telark/internal/data/auth"
)

type (
	LoginStartRequest struct {
		Email string `json:"email"`
	}
	LoginStartResponse struct {
		Options *protocol.CredentialAssertion `json:"options"`
		UserID  string                        `json:"userId,omitempty"`
	}
	LoginFinishRequest struct {
		// The WebAuthn credential rides in the same body; FinishLogin parses it
		// straight off the raw request, so it is not modeled here.
		Email string `json:"email"`
		authdata.DeviceMetadata
	}
	LoginFinishResponse struct {
		SessionToken string `json:"sessionToken"`
		User         any    `json:"user"`
	}
)
