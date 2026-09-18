package session

import (
	"net/http"

	"github.com/telark/data/errors"
	"github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/constants"
	sessionexp "github.com/telark/exporter/internal/exporters/auth/session"
	sessionutils "github.com/telark/exporter/internal/utils/auth/session"
	authutils "github.com/telark/exporter/internal/utils/auth/shared"
	"github.com/telark/exporter/internal/utils/performance"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/rest/response"
	requestutils "github.com/telark/rest/utils/request"
)

func CreateSessionByUserWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := sharedutils.GetPathParam(w, r, constants.UserIDParam)
		if err != nil {
			return
		}

		body, err := requestutils.ParseRequestBody(r)
		if err != nil {
			sharedutils.LogByStatusAndSend(
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

		if !authz.GuardSelfUser(w, r, userID) {
			return
		}

		sessionexp.ListSessionsByUser(w, userID)
	}
}

func GetSessionByToken() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := sessionutils.RefFromRequest(w, r)
		if !ok {
			return
		}

		if !authz.GuardSelfSessionToken(w, r, token) {
			return
		}

		sessionexp.GetSessionByToken(w, token)
	}
}

func PatchSessionByTokenWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := sessionutils.RefFromRequest(w, r)
		if !ok {
			return
		}

		body, err := requestutils.ParseRequestBody(r)
		if err != nil {
			sharedutils.LogByStatusAndSend(
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
		token, ok := sessionutils.RefFromRequest(w, r)
		if !ok {
			return
		}

		if !authz.GuardSelfSessionToken(w, r, token) {
			return
		}

		sessionexp.DeleteSessionByToken(w, token)
		authutils.InvalidateResourceCaches(optimizer, constants.ResourceUserSession, string(constants.OpDelete), constants.EmptyString)
	}
}
