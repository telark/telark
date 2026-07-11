package auth

import (
	"fmt"
	"net/http"
	"time"

	"github.com/telark/auth/internal/clients"
	"github.com/telark/auth/internal/constants"
	authhelper "github.com/telark/auth/internal/helpers/auth"
	"github.com/telark/auth/internal/helpers/shared"
	authdata "github.com/telark/data/auth"
)

func Logout(w http.ResponseWriter, r *http.Request) {
	userID := constants.EmptyString
	tokenStatus := constants.TokenStatusInvalid

	if sessionToken, err := authhelper.ExtractSessionToken(r); err == nil {
		userID, tokenStatus = revokeSessionIfPresent(sessionToken)
	}

	lg.Info(fmt.Sprintf(string(constants.LogLogoutAttempted), userID, tokenStatus))
	shared.SendSuccessResponse(w, string(constants.SuccessLogoutCompleted), nil)
}

func revokeSessionIfPresent(sessionToken string) (userID string, tokenStatus string) {
	sessionClient := clients.GetSessionClient()
	session, err := sessionClient.GetSessionByToken(sessionToken)
	if err != nil || session == nil {
		return constants.EmptyString, constants.TokenStatusInvalid
	}

	sessionClient.DeleteSessionByToken(sessionToken)
	return session.UserID, resolveTokenStatus(session)
}

func resolveTokenStatus(session *authdata.UserSession) string {
	expiresAt, err := time.Parse(constants.TimeFormatRFC3339, session.ExpiresTimestamp)
	if err != nil || time.Now().After(expiresAt) {
		return constants.TokenStatusExpired
	}
	return constants.TokenStatusValid
}
