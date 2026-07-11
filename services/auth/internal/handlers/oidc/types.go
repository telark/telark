package oidc

import (
	authdata "github.com/telark/data/auth"
	userresource "github.com/telark/data/resources/user"
)

type CallbackRequest struct {
	IDToken string `json:"idToken"`
	authdata.DeviceMetadata
}

type CallbackResponse struct {
	SessionToken string                       `json:"sessionToken"`
	Email        string                       `json:"email"`
	User         *userresource.UserAsResource `json:"user,omitempty"`
}

type NonceResponse struct {
	Nonce string `json:"nonce"`
}
