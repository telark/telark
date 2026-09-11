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

func RegisterStart(w http.ResponseWriter, r *http.Request) {
	user, userID, err := authhelper.GetUserForRegistrationStart(r)
	if err != nil {
		statusCode := http.StatusBadRequest
		if shared.IsError(err, constants.ErrUserNotFound) {
			statusCode = http.StatusNotFound
		} else if shared.IsError(err, constants.ErrUserAlreadyHasPasskeysPleaseLoginFirst) {
			statusCode = http.StatusUnauthorized
		}
		shared.SendErrorResponse(w, statusCode, err)
		return
	}

	existingCredentials := []webauthn.Credential{}
	options, _, err := webauthnhelper.StartRegistration(userID, user.Username, user.Fullname, existingCredentials)
	if err != nil {
		shared.HandleError(w, err, http.StatusInternalServerError, err.Error())
		return
	}

	shared.SendJSONResponse(w, http.StatusOK, RegisterStartResponse{Options: options})
}
