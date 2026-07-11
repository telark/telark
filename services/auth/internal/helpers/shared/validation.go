package shared

import (
	"errors"
	"net/mail"

	"github.com/telark/auth/internal/constants"
)

func ValidateUsername(username string) error {
	if username == constants.EmptyString {
		return errors.New(string(constants.ErrInvalidUsername))
	}
	return nil
}

func ValidateEmail(email string) error {
	if email == constants.EmptyString {
		return errors.New(string(constants.ErrInvalidEmail))
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return errors.New(string(constants.ErrInvalidEmail))
	}
	return nil
}
