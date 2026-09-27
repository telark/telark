package passkey

import (
	"net/http"

	"github.com/telark/data/errors"
	"github.com/telark/exporter/internal/constants"
	passkeyexp "github.com/telark/exporter/internal/exporters/auth/passkey"
	passkeyutils "github.com/telark/exporter/internal/utils/auth/passkey"
	authutils "github.com/telark/exporter/internal/utils/auth/shared"
	"github.com/telark/exporter/internal/utils/performance"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/rest/response"
	requestutils "github.com/telark/rest/utils/request"
)

func CreatePasskeyByUserWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := sharedutils.GetHeader(w, r, constants.HeaderUserID)
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

		passkeyexp.CreatePasskeyByUser(w, body, userID)
		authutils.InvalidateResourceCaches(optimizer, constants.ResourceUserPasskey, string(constants.OpCreate), constants.EmptyString)
	}
}

func ListPasskeysByUserWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := sharedutils.GetHeader(w, r, constants.HeaderUserID)
		if err != nil {
			return
		}

		passkeyexp.ListPasskeysByUser(w, userID)
	}
}

func GetPasskeyByUserAndCredentialIDWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := sharedutils.GetHeader(w, r, constants.HeaderUserID)
		if err != nil {
			return
		}

		credentialID, err := sharedutils.GetPathParam(w, r, constants.CredentialIDParam)
		if err != nil {
			return
		}

		passkeyexp.GetPasskeyByCredentialID(w, credentialID, userID)
	}
}

func PatchPasskeyByUserAndCredentialIDWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, credentialID, body, ok := passkeyutils.ExtractPasskeyRequestParams(w, r)
		if !ok {
			return
		}

		passkeyexp.PatchPasskeyByCredentialID(w, credentialID, userID, body)
		authutils.InvalidateResourceCaches(optimizer, constants.ResourceUserPasskey, string(constants.OpPatch), constants.EmptyString)
	}
}

func DeletePasskeyByUserAndCredentialIDWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, credentialID, body, ok := passkeyutils.ExtractPasskeyRequestParams(w, r)
		if !ok {
			return
		}

		forceLastDelete := false
		if forceLastDeleteVal, ok := body["forceLastDelete"].(bool); ok {
			forceLastDelete = forceLastDeleteVal
		}

		passkeyexp.DeletePasskeyByCredentialID(w, credentialID, userID, forceLastDelete)
		authutils.InvalidateResourceCaches(optimizer, constants.ResourceUserPasskey, string(constants.OpDelete), constants.EmptyString)
	}
}
