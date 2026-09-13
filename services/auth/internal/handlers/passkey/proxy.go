package passkey

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/telark/auth/internal/clients"
	"github.com/telark/auth/internal/constants"
	authhelper "github.com/telark/auth/internal/helpers/auth"
	"github.com/telark/auth/internal/helpers/shared"
	webauthnhelper "github.com/telark/auth/internal/helpers/webauthn"
	xauthz "github.com/telark/x-ware/authz"
)

func GetPasskeys(w http.ResponseWriter, r *http.Request) {
	userID, err := authhelper.ValidateSessionFromRequest(r)
	if err != nil {
		shared.SendErrorResponse(w, http.StatusUnauthorized, err)
		return
	}

	passkeyClient := clients.GetPasskeyClient()
	passkeys, err := passkeyClient.GetAllPasskeysByUser(userID)
	if err != nil {
		shared.HandleError(w, errors.New(string(constants.ErrInternalServerError)),
			http.StatusInternalServerError,
			fmt.Sprintf(string(constants.ErrFailedProxyRequest), err))
		return
	}

	shared.SendJSONResponse(w, http.StatusOK, passkeys)
}

func CreatePasskey(w http.ResponseWriter, r *http.Request) {
	user, userID, err := authhelper.GetUserForRegistration(r, webauthnhelper.RegistrationChallengeOwner)
	if err != nil {
		statusCode := http.StatusBadRequest
		if shared.IsError(err, constants.ErrUserAlreadyHasPasskeys) {
			statusCode = http.StatusUnauthorized
		}
		shared.SendErrorResponse(w, statusCode, err)
		return
	}

	deviceName, deviceType, err := authhelper.ValidateDeviceHeaders(r)
	if err != nil {
		shared.SendErrorResponse(w, http.StatusBadRequest, err)
		return
	}

	credential, backupEligible, backupState, err := webauthnhelper.FinishRegistration(
		userID, user.Username, user.Fullname, r)
	if err != nil {
		shared.SendErrorResponse(w, http.StatusUnauthorized, err)
		return
	}

	if err := authhelper.AttachPasskeyIdentity(userID, user, credential); err != nil {
		shared.HandleError(w, errors.New(string(constants.ErrInternalServerError)),
			http.StatusInternalServerError, err.Error())
		return
	}

	passkey := authhelper.CreatePasskeyFromCredential(
		userID, credential, deviceName, deviceType, backupEligible, backupState)

	data, err := authhelper.CreatePasskey(userID, passkey)
	if err != nil {
		// Check if it's a conflict error (409)
		var conflictErr *authhelper.ConflictError
		if errors.As(err, &conflictErr) {
			shared.SendErrorResponse(w, http.StatusConflict, err)
			return
		}
		shared.HandleError(w, errors.New(string(constants.ErrInternalServerError)),
			http.StatusInternalServerError, err.Error())
		return
	}

	shared.SendJSONResponse(w, http.StatusCreated, data)
}

func GetSinglePasskey(w http.ResponseWriter, r *http.Request) {
	userID, credentialID, err := authhelper.ValidateSessionAndExtractCredentialID(r)
	if err != nil {
		shared.SendErrorResponse(w, shared.GetStatusCodeForAuthError(err), err)
		return
	}

	passkeyClient := clients.GetPasskeyClient()
	passkey, err := passkeyClient.GetPasskeyByUserAndCredentialID(userID, credentialID)
	if err != nil {
		shared.HandleError(w, errors.New(string(constants.ErrPasskeyNotFound)),
			http.StatusNotFound,
			fmt.Sprintf(string(constants.ErrFailedProxyRequest), err))
		return
	}

	shared.SendJSONResponse(w, http.StatusOK, passkey)
}

func UpdatePasskey(w http.ResponseWriter, r *http.Request) {
	userID, credentialID, err := authhelper.ValidateSessionAndExtractCredentialID(r)
	if err != nil {
		shared.SendErrorResponse(w, shared.GetStatusCodeForAuthError(err), err)
		return
	}

	var req UpdatePasskeyRequest
	if err := shared.DecodeRequestBody(r, &req); err != nil {
		shared.SendErrorResponse(w, http.StatusBadRequest, err)
		return
	}

	updateData := make(map[string]any, constants.MaxUpdateFields)
	if req.DeviceName != nil {
		updateData[constants.PasskeyFieldDeviceName] = *req.DeviceName
	}
	if req.LastUsedTimestamp != nil {
		updateData[constants.PasskeyFieldLastUsedTimestamp] = *req.LastUsedTimestamp
	}

	data, err := authhelper.UpdatePasskey(userID, credentialID, updateData)
	if err != nil {
		shared.HandleError(w, errors.New(string(constants.ErrInternalServerError)),
			http.StatusInternalServerError, err.Error())
		return
	}

	shared.SendJSONResponse(w, http.StatusOK, data)
}

// Orphan cleanup names a user who cannot produce a session, so only a peer
// service may name one. Every other caller is bound to its own session: a
// request header can never decide whose credential is deleted.
func resolveDeleteTarget(r *http.Request, cleanupOrphaned bool) (userID, credentialID string, status int, err error) {
	identity, found := xauthz.FromContext(r.Context())
	if !cleanupOrphaned || !found || !identity.Internal {
		userID, credentialID, err = authhelper.ValidateSessionAndExtractCredentialID(r)
		if err != nil {
			return constants.EmptyString, constants.EmptyString, shared.GetStatusCodeForAuthError(err), err
		}
		return userID, credentialID, http.StatusOK, nil
	}

	userID = r.Header.Get(constants.HeaderUserID)
	if userID == constants.EmptyString {
		return constants.EmptyString, constants.EmptyString, http.StatusBadRequest,
			errors.New(string(constants.ErrMissingUserID))
	}

	credentialID, err = authhelper.ExtractCredentialID(r)
	if err != nil {
		return constants.EmptyString, constants.EmptyString, http.StatusBadRequest, err
	}

	return userID, credentialID, http.StatusOK, nil
}

func DeletePasskey(w http.ResponseWriter, r *http.Request) {
	var req DeletePasskeyRequest
	forceLastDelete := constants.DefaultForceLastDelete
	cleanupOrphaned := constants.DefaultCleanupOrphaned
	if err := shared.DecodeRequestBody(r, &req); err != nil {
		lg.Debug(fmt.Sprintf(string(constants.ErrFailedDecodeRequest), err))
	} else {
		forceLastDelete = req.ForceLastDelete
		cleanupOrphaned = req.CleanupOrphaned
	}

	userID, credentialID, status, err := resolveDeleteTarget(r, cleanupOrphaned)
	if err != nil {
		shared.SendErrorResponse(w, status, err)
		return
	}

	if err := authhelper.DeletePasskey(userID, credentialID, forceLastDelete); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), string(constants.ErrPasskeyNotFound)) {
			shared.SendErrorResponse(w, http.StatusNotFound, errors.New(string(constants.ErrPasskeyNotFound)))
			return
		}
		shared.HandleError(w, errors.New(string(constants.ErrInternalServerError)),
			http.StatusInternalServerError, err.Error())
		return
	}

	shared.SendSuccessResponse(w, string(constants.SuccessPasskeyDeleted), nil)
}
