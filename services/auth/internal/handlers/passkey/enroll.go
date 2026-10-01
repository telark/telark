package passkey

import (
	"errors"
	"fmt"
	"net/http"

	dataerrors "github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/services/auth/internal/authz"
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

// The token leaves auth only in this answer; the UI builds the link from it.
func CreateUserEnrollLink(w http.ResponseWriter, r *http.Request) {
	targetID, ok := enrollTarget(w, r)
	if !ok {
		return
	}
	if status, err := authz.GuardEnrollLinkIssue(r.Context(), targetID); err != nil {
		shared.SendErrorResponse(w, status, err)
		return
	}

	issuerID := r.Header.Get(constants.HeaderUserID)
	token, expiresAt, err := authhelper.IssueInvite(targetID, issuerID)
	if err != nil {
		shared.HandleError(w, errors.New(string(constants.ErrInternalServerError)),
			http.StatusInternalServerError, err.Error())
		return
	}

	lg.Info(fmt.Sprintf(string(constants.LogEnrollLinkIssued), shared.IdentityHash(targetID), shared.IdentityHash(issuerID)))
	shared.SendJSONResponse(w, http.StatusCreated, EnrollLinkResponse{
		Token:     token,
		ExpiresAt: expiresAt.Format(constants.TimeFormatRFC3339),
	})
}

func RevokeUserEnrollLink(w http.ResponseWriter, r *http.Request) {
	targetID, ok := enrollTarget(w, r)
	if !ok {
		return
	}
	if status, err := authz.GuardEnrollLinkRevoke(r.Context(), targetID); err != nil {
		shared.SendErrorResponse(w, status, err)
		return
	}

	if err := authhelper.RevokeInvite(targetID); err != nil {
		shared.HandleError(w, errors.New(string(constants.ErrInternalServerError)),
			http.StatusInternalServerError, err.Error())
		return
	}

	lg.Info(fmt.Sprintf(string(constants.LogEnrollLinkRevoked), shared.IdentityHash(targetID)))
	shared.SendSuccessResponse(w, string(constants.SuccessEnrollLinkRevoked), nil)
}

func enrollTarget(w http.ResponseWriter, r *http.Request) (string, bool) {
	targetID, err := shared.GetPathParam(r, constants.IDPathParam)
	if err != nil || targetID == constants.EmptyString {
		shared.SendErrorResponse(w, http.StatusBadRequest, fmt.Errorf(string(dataerrors.ErrRestRequiredParam), constants.IDPathParam))
		return constants.EmptyString, false
	}
	return targetID, true
}
