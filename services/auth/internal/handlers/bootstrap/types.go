package bootstrap

import "github.com/go-webauthn/webauthn/protocol"

type StartResponse struct {
	Options *protocol.CredentialAssertion `json:"options"`
}
