package passkey

import (
	"errors"
	"net/http"

	"github.com/telark/telark/services/auth/internal/constants"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
	"github.com/telark/telark/services/auth/internal/helpers/shared"
)

func CreateEnrollLink(w http.ResponseWriter, r *http.Request) {
	userID, err := authhelper.ValidateSessionFromRequest(r)
	if err != nil {
		shared.SendErrorResponse(w, http.StatusUnauthorized, err)
		return
	}

	token, expiresAt, err := authhelper.CreateEnrollToken(userID)
	if err != nil {
		shared.HandleError(w, errors.New(string(constants.ErrInternalServerError)),
			http.StatusInternalServerError, err.Error())
		return
	}

	shared.SendJSONResponse(w, http.StatusCreated, EnrollLinkResponse{
		Token:     token,
		ExpiresAt: expiresAt.Format(constants.TimeFormatRFC3339),
	})
}
