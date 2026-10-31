package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/telark/telark/services/auth/internal/bootstrap"
	"github.com/telark/telark/services/auth/internal/constants"
	"github.com/telark/telark/services/auth/internal/helpers/auth"
	"github.com/telark/telark/services/auth/internal/helpers/shared"
	webauthnhelper "github.com/telark/telark/services/auth/internal/helpers/webauthn"
)

var lg = constants.GetLogger(constants.LoggerPrefixHandler)

func LoginStart(w http.ResponseWriter, r *http.Request) {
	var req LoginStartRequest
	if err := shared.DecodeRequestBody(r, &req); err != nil {
		shared.SendErrorResponse(w, http.StatusBadRequest, err)
		return
	}

	email, err := shared.ValidateEmail(req.Email)
	if err != nil {
		shared.SendErrorResponse(w, http.StatusBadRequest, err)
		return
	}
	req.Email = email

	user, passkeys, err := auth.GetUserAndPasskeys(req.Email)
	if err != nil {
		shared.SendErrorResponse(w, userLookupStatus(err), err)
		return
	}

	credentials := webauthnhelper.ConvertPasskeysToCredentials(passkeys)
	webAuthnUser := webauthnhelper.CreateUser(user.ID, user.Username, user.Fullname, credentials)

	wa, err := webauthnhelper.GetWebAuthnFor(r)
	if err != nil {
		shared.HandleError(w, err, shared.GetStatusCodeForWebAuthnError(err, http.StatusInternalServerError), err.Error())
		return
	}

	options, sessionData, err := wa.BeginLogin(webAuthnUser)
	if err != nil {
		shared.HandleError(w, fmt.Errorf(string(constants.ErrChallengeGenerationFailed), err),
			http.StatusInternalServerError,
			fmt.Sprintf(string(constants.ErrChallengeGenerationFailed), err))
		return
	}

	if err := webauthnhelper.StoreChallenge(user.ID, sessionData.Challenge, constants.EmptyString); err != nil {
		if shared.IsError(err, constants.ErrAccountRecovering) {
			shared.SendErrorResponse(w, http.StatusConflict, err)
			return
		}
		shared.HandleError(w, errors.New(string(constants.ErrInternalServerError)),
			http.StatusInternalServerError,
			err.Error())
		return
	}

	response := LoginStartResponse{
		Options: options,
		UserID:  user.ID,
	}
	shared.SendJSONResponse(w, http.StatusOK, response)
}

func LoginFinish(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := webauthnhelper.ReadAndRestoreRequestBody(r)
	if err != nil {
		shared.SendErrorResponse(w, http.StatusBadRequest, err)
		return
	}

	req, err := extractLoginRequest(bodyBytes)
	if err != nil {
		shared.SendErrorResponse(w, http.StatusBadRequest, err)
		return
	}

	user, passkeys, err := auth.GetUserAndPasskeys(req.Email)
	if err != nil {
		shared.SendErrorResponse(w, userLookupStatus(err), err)
		return
	}

	challenge, err := webauthnhelper.ValidateAndGetChallenge(user.ID, bodyBytes)
	if err != nil {
		sendChallengeError(w, err)
		return
	}

	credentials := webauthnhelper.ConvertPasskeysToCredentials(passkeys)
	webAuthnUser := webauthnhelper.CreateUser(user.ID, user.Username, user.Fullname, credentials)
	credential, err := webauthnhelper.VerifyCredential(challenge, webAuthnUser, credentials, passkeys, r, protocol.VerificationPreferred)
	if err != nil {
		shared.SendErrorResponse(w, shared.GetStatusCodeForWebAuthnError(err, http.StatusUnauthorized), err)
		return
	}
	if err := bootstrap.RefuseRecoveryPasskey(user, passkeys, credential.ID); err != nil {
		sendRecoveryRefusal(w, err)
		return
	}

	capturedUserID, capturedCredID := user.ID, credential.ID
	auth.Dispatch(func() { _ = auth.UpdatePasskeyLastUsed(capturedUserID, capturedCredID) })

	sessionToken, expiresAt, err := auth.CreateUserSession(user.ID, &req.DeviceMetadata)
	if err != nil {
		sendSessionError(w, err)
		return
	}
	auth.SetSessionCookie(w, sessionToken, expiresAt)

	// A refused login is no login: the stamp waits for the session, as on the Google path. The bootstrap account's
	// stamp is what stops a break-glass request completing without approval, so it never waits on the pool.
	if user.Bootstrap {
		auth.UpdateUserLastLogin(capturedUserID)
		bootstrap.CancelOnSignIn(user.ID)
	} else {
		auth.Dispatch(func() { auth.UpdateUserLastLogin(capturedUserID) })
	}

	shared.SendJSONResponse(w, http.StatusOK, LoginFinishResponse{User: user})
}

// An expired or already used challenge is the caller's; Redis failing to answer is logged.
func sendChallengeError(w http.ResponseWriter, err error) {
	if shared.IsError(err, constants.ErrChallengeNotFound) {
		shared.SendErrorResponse(w, http.StatusUnauthorized, err)
		return
	}
	if shared.IsError(err, constants.ErrAccountRecovering) {
		shared.SendErrorResponse(w, http.StatusConflict, err)
		return
	}
	shared.HandleError(w, err, http.StatusUnauthorized, err.Error())
}

func sendRecoveryRefusal(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, bootstrap.ErrRecoveryPasskey):
		shared.SendErrorResponse(w, http.StatusConflict, err)
	case errors.Is(err, shared.ErrBackendUnavailable):
		shared.SendErrorResponse(w, http.StatusServiceUnavailable, err)
	default:
		shared.HandleError(w, errors.New(string(constants.ErrInternalServerError)), http.StatusInternalServerError, err.Error())
	}
}

// A refused account is an answer; only a session that could not be made is logged, and its cause stays out of the reply.
func sendSessionError(w http.ResponseWriter, err error) {
	status := shared.GetStatusCodeForSessionError(err)
	if status != http.StatusInternalServerError {
		shared.SendErrorResponse(w, status, err)
		return
	}
	shared.HandleError(w, errors.New(string(constants.ErrInternalServerError)), status, err.Error())
}

func userLookupStatus(err error) int {
	if shared.IsError(err, constants.ErrUserNotFound) || shared.IsError(err, constants.ErrNoPasskeysFound) {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}

func extractLoginRequest(bodyBytes []byte) (*LoginFinishRequest, error) {
	var req LoginFinishRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedDecodeRequest), err)
	}
	email, err := shared.ValidateEmail(req.Email)
	if err != nil {
		return nil, err
	}
	req.Email = email
	return &req, nil
}
