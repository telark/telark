package session

import (
	"net/http"

	authdata "github.com/telark/telark/internal/data/auth"
	authendpoints "github.com/telark/telark/internal/rest/endpoints/auth"
	"github.com/telark/telark/internal/rest/response"
	"github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
	sessionexp "github.com/telark/telark/services/exporter/internal/exporters/auth/session"
	sessionutils "github.com/telark/telark/services/exporter/internal/utils/auth/session"
	authutils "github.com/telark/telark/services/exporter/internal/utils/auth/shared"
	"github.com/telark/telark/services/exporter/internal/utils/performance"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

func CreateSessionByUserWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := sharedutils.GetPathParam(w, r, constants.UserIDParam)
		if err != nil {
			return
		}

		body, err := sharedutils.GetSpec(w, r)
		if err != nil {
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

		body, err := sharedutils.GetSpec(w, r)
		if err != nil {
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
