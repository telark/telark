package shared

import (
	"errors"
	"net/mail"

	"github.com/telark/telark/internal/data/text"
	"github.com/telark/telark/services/auth/internal/constants"
)

// The trimmed address; net/mail also accepts "Name <a@b>" forms, which are refused.
func ValidateEmail(email string) (string, error) {
	email, err := text.Check(email, text.Rule{Field: constants.RequestFieldEmail,
		Min: constants.DefaultIncrementValue, Max: constants.EmailMaxLen})
	if err != nil {
		return constants.EmptyString, err
	}
	if addr, err := mail.ParseAddress(email); err != nil || addr.Name != constants.EmptyString || addr.Address != email {
		return constants.EmptyString, errors.New(string(constants.ErrInvalidEmail))
	}
	return email, nil
}
