package oidc

import (
	"encoding/json"

	authdata "github.com/telark/data/auth"
	telarkconfigresource "github.com/telark/data/resources/telarkconfig"
	userresource "github.com/telark/data/resources/user"
)

type CallbackRequest struct {
	IDToken string `json:"idToken"`
	authdata.DeviceMetadata
}

type CallbackResponse struct {
	SessionToken string             `json:"sessionToken"`
	Email        string             `json:"email"`
	User         *userresource.User `json:"user,omitempty"`
}

type NonceResponse struct {
	Nonce string `json:"nonce"`
}

// GoogleJWKJSON shadows the embedded omitempty field: an omitted key keeps the
// stored set, while "" or null clears it, and only the raw value tells them apart.
type SetConfigRequest struct {
	telarkconfigresource.OIDCConfig
	GoogleJWKJSON json.RawMessage `json:"googleJwkJson,omitempty"`
}
