package passkey

import (
	"net/http"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/telark/auth/internal/constants"
	authhelper "github.com/telark/auth/internal/helpers/auth"
	"github.com/telark/auth/internal/helpers/shared"
	webauthnhelper "github.com/telark/auth/internal/helpers/webauthn"
)

var lg = constants.GetLogger(constants.LoggerPrefixHandler)

func registerStartStatus(err error) int {
	switch {
	case shared.IsError(err, constants.ErrUserNotFound):
		return http.StatusNotFound
	case shared.IsError(err, constants.ErrUserAlreadyHasPasskeysPleaseLoginFirst),
		shared.IsError(err, constants.ErrEnrollTokenInvalid):
		return http.StatusUnauthorized
	case shared.IsError(err, constants.ErrRegisterEmailMismatch):
		return http.StatusForbidden
	default:
		return http.StatusBadRequest
	}
}

func RegisterStart(w http.ResponseWriter, r *http.Request) {
	user, userID, enrolled, err := authhelper.GetUserForRegistrationStart(r)
	if err != nil {
		shared.SendErrorResponse(w, registerStartStatus(err), err)
		return
	}

	existingCredentials := []webauthn.Credential{}
	options, challenge, err := webauthnhelper.StartRegistration(userID, user.Username, user.Fullname, existingCredentials, r)
	if err != nil {
		shared.HandleError(w, err, shared.GetStatusCodeForWebAuthnError(err, http.StatusInternalServerError), err.Error())
		return
	}

	if enrolled {
		if err := webauthnhelper.StoreEnrolledCeremony(challenge, userID); err != nil {
			shared.HandleError(w, err, http.StatusInternalServerError, err.Error())
			return
		}
	}

	shared.SendJSONResponse(w, http.StatusOK, RegisterStartResponse{Options: options})
}
