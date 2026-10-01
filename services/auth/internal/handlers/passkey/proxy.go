package passkey

import (
	"errors"
	"fmt"
	"net/http"

	requestutils "github.com/telark/telark/internal/rest/utils/request"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/auth/internal/clients"
	"github.com/telark/telark/services/auth/internal/constants"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
	"github.com/telark/telark/services/auth/internal/helpers/shared"
	webauthnhelper "github.com/telark/telark/services/auth/internal/helpers/webauthn"
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
		shared.SendErrorResponse(w, shared.GetStatusCodeForWebAuthnError(err, http.StatusUnauthorized), err)
		return
	}

	// A self-registration is saved only now that its credential verified.
	rollback := func() {}
	if user.ID == constants.EmptyString {
		if user, err = authhelper.CreatePendingUser(user); err != nil {
			shared.SendErrorResponse(w, registrationStatus(err), err)
			return
		}
		userID = user.ID
		rollback = func() { authhelper.DiscardPendingUser(userID) }
	}

	if err := authhelper.AttachPasskeyIdentity(userID, user, credential); err != nil {
		rollback()
		shared.HandleError(w, errors.New(string(constants.ErrInternalServerError)),
			http.StatusInternalServerError, err.Error())
		return
	}

	passkey := authhelper.CreatePasskeyFromCredential(
		userID, credential, deviceName, deviceType, backupEligible, backupState)

	data, err := authhelper.CreatePasskey(userID, passkey)
	if err != nil {
		rollback()
		sendProxyError(w, err)
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
		sendProxyError(w, err)
		return
	}

	shared.SendJSONResponse(w, http.StatusOK, data)
}

// Only a peer service may name the user (orphan cleanup has no session); every
// other caller is bound to its own session, never to a request header.
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
		// The body is optional, but one over the cap is refused as on every other route.
		if errors.Is(err, requestutils.ErrRequestBodyTooLarge) {
			shared.SendErrorResponse(w, http.StatusRequestEntityTooLarge, err)
			return
		}
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
		sendProxyError(w, err)
		return
	}

	shared.SendSuccessResponse(w, string(constants.SuccessPasskeyDeleted), nil)
}

// A 4xx is the exporter's verdict on this request and is relayed as is; a 5xx
// stays an opaque internal error.
func sendProxyError(w http.ResponseWriter, err error) {
	proxyErr, ok := errors.AsType[*authhelper.ProxyError](err)
	if ok && proxyErr.Status < http.StatusInternalServerError {
		shared.SendErrorResponse(w, proxyErr.Status, err)
		return
	}
	shared.HandleError(w, errors.New(string(constants.ErrInternalServerError)),
		http.StatusInternalServerError, fmt.Sprintf(string(constants.ErrFailedProxyRequest), err))
}
