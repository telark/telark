package auth

import (
	"github.com/go-webauthn/webauthn/protocol"
	authdata "github.com/telark/data/auth"
)

type (
	LoginStartRequest struct {
		Email string `json:"email"`
	}
	LoginStartResponse struct {
		Options *protocol.CredentialAssertion `json:"options"`
		UserID  string                         `json:"userId,omitempty"`
	}
	LoginFinishRequest struct {
		/*
			Note: The credential is sent in the request body in WebAuthn format
			The go-webauthn library's FinishLogin will parse it from the raw request
		*/
		Email string `json:"email"`
		authdata.DeviceMetadata
	}
	LoginFinishResponse struct {
		SessionToken string `json:"sessionToken"`
		User         any    `json:"user"`
	}
)
