package session

import (
	"fmt"
	"net/http"

	authdata "github.com/telark/data/auth"
	"github.com/telark/data/errors"
	"github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/constants"
	sessionexp "github.com/telark/exporter/internal/exporters/auth/session"
	sessionutils "github.com/telark/exporter/internal/utils/auth/session"
	authutils "github.com/telark/exporter/internal/utils/auth/shared"
	"github.com/telark/exporter/internal/utils/performance"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	authendpoints "github.com/telark/rest/endpoints/auth"
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
				fmt.Sprintf(string(errors.ErrRestParseRequestBody), err),
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
		userID, err := sharedutils.GetQueryParam(w, r, authendpoints.QuerySessionUser)
		if err != nil {
			return
		}

		if !authz.GuardSelfUser(w, r, userID) {
			return
		}

		sessionexp.ListSessionsByUser(w, userID)
	}
}

func GetSelfSession() func(http.ResponseWriter, *http.Request) {
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

func PatchSelfSessionWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
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
				fmt.Sprintf(string(errors.ErrRestParseRequestBody), err),
				nil,
				err,
			)
			return
		}

		sessionexp.PatchSessionByToken(w, token, body)
		authutils.InvalidateResourceCaches(optimizer, constants.ResourceUserSession, string(constants.OpPatch), constants.EmptyString)
	}
}

func DeleteSelfSessionWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
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

// Revokes one of the caller's other sessions by name; the self routes cover the
// session making the request.
func DeleteSessionByNameWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		name, err := sharedutils.GetPathParam(w, r, constants.NameParam)
		if err != nil {
			return
		}

		if !authdata.IsSessionName(name) {
			sharedutils.LogByStatusAndSend(w, http.StatusBadRequest, response.OperationError,
				string(constants.ErrSessionNameInvalid), nil, nil)
			return
		}

		if !authz.GuardOwnSessionName(w, r, name) {
			return
		}

		sessionexp.DeleteSessionByToken(w, name)
		authutils.InvalidateResourceCaches(optimizer, constants.ResourceUserSession, string(constants.OpDelete), constants.EmptyString)
	}
}
