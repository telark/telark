package webauthn

import (
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/telark/telark/services/auth/internal/constants"
)

type User struct {
	ID          []byte
	Name        string
	DisplayName string
	Credentials []webauthn.Credential
}

func (u *User) WebAuthnID() []byte {
	return u.ID
}

func (u *User) WebAuthnName() string {
	return u.Name
}

func (u *User) WebAuthnDisplayName() string {
	return u.DisplayName
}

func (*User) WebAuthnIcon() string {
	return constants.EmptyString
}

func (u *User) WebAuthnCredentials() []webauthn.Credential {
	return u.Credentials
}
