package session

import (
	"net/http"

	"github.com/telark/data/errors"
	"github.com/telark/exporter/internal/constants"
	sessionexp "github.com/telark/exporter/internal/exporters/auth/session"
	authutils "github.com/telark/exporter/internal/utils/auth/shared"
	"github.com/telark/exporter/internal/utils/performance"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/rest/response"
	requestutils "github.com/telark/rest/utils/request"
	responseutils "github.com/telark/rest/utils/response"
)

func CreateSessionByUserWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := sharedutils.GetPathParam(w, r, constants.UserIDParam)
		if err != nil {
			return
		}

		body, err := requestutils.ParseRequestBody(r)
		if err != nil {
			responseutils.LogAndSendResponse(
				w,
				http.StatusUnprocessableEntity,
				response.OperationUnprocessed,
				string(errors.ErrRestParseRequestBody),
				nil,
				err,
			)
			return
		}

		sessionexp.CreateSessionByUser(w, body, userID)
		authutils.InvalidateResourceCaches(optimizer, constants.ResourceUserSession, string(constants.OpCreate), constants.EmptyString)
	}
}

func ListSessionsByUserWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := sharedutils.GetPathParam(w, r, constants.UserIDParam)
		if err != nil {
			return
		}

		sessionexp.ListSessionsByUser(w, userID)
	}
}

func GetSessionByToken() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		token, err := sharedutils.GetPathParam(w, r, constants.TokenParam)
		if err != nil {
			return
		}

		sessionexp.GetSessionByToken(w, token)
	}
}

func PatchSessionByTokenWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		token, err := sharedutils.GetPathParam(w, r, constants.TokenParam)
		if err != nil {
			return
		}

		body, err := requestutils.ParseRequestBody(r)
		if err != nil {
			responseutils.LogAndSendResponse(
				w,
				http.StatusUnprocessableEntity,
				response.OperationUnprocessed,
				string(errors.ErrRestParseRequestBody),
				nil,
				err,
			)
			return
		}

		sessionexp.PatchSessionByToken(w, token, body)
		authutils.InvalidateResourceCaches(optimizer, constants.ResourceUserSession, string(constants.OpPatch), constants.EmptyString)
	}
}

func DeleteSessionByTokenWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		token, err := sharedutils.GetPathParam(w, r, constants.TokenParam)
		if err != nil {
			return
		}

		sessionexp.DeleteSessionByToken(w, token)
		authutils.InvalidateResourceCaches(optimizer, constants.ResourceUserSession, string(constants.OpDelete), constants.EmptyString)
	}
}
