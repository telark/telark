package authrequest

import (
	"github.com/telark/telark/internal/data/resources/authrequest"
	"github.com/telark/telark/services/auth/internal/constants"
)

// One request's run. final is the status it ends with: the first failure, unless a later step names the end itself.
type run struct {
	request   authrequest.AuthRequest
	email     string
	lock      string
	requestID string
	key       *[constants.AuthRequestKeyBytes]byte
	final     authrequest.Status
}
