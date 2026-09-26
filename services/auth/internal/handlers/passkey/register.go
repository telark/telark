package passkey

import (
	"net/http"

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
	case shared.IsError(err, constants.ErrUserAlreadyHasPasskeys),
		shared.IsError(err, constants.ErrRegistrationNeedsProof),
		shared.IsError(err, constants.ErrEnrollTokenInvalid):
		return http.StatusUnauthorized
	case shared.IsError(err, constants.ErrRegisterEmailMismatch),
		shared.IsError(err, constants.ErrReservedEmail),
		shared.IsError(err, constants.ErrSelfRegistrationDisabled):
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

	options, challenge, err := webauthnhelper.StartRegistration(userID, user.Username, user.Fullname, nil, r)
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
