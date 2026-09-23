package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/telark/auth/internal/constants"
	"github.com/telark/auth/internal/helpers/auth"
	"github.com/telark/auth/internal/helpers/shared"
	webauthnhelper "github.com/telark/auth/internal/helpers/webauthn"
)

var lg = constants.GetLogger(constants.LoggerPrefixHandler)

func LoginStart(w http.ResponseWriter, r *http.Request) {
	var req LoginStartRequest
	if err := shared.DecodeRequestBody(r, &req); err != nil {
		shared.SendErrorResponse(w, http.StatusBadRequest, err)
		return
	}

	if err := shared.ValidateEmail(req.Email); err != nil {
		shared.SendErrorResponse(w, http.StatusBadRequest, err)
		return
	}

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

	if err := webauthnhelper.StoreChallenge(user.ID, sessionData.Challenge); err != nil {
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
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		shared.SendErrorResponse(w, http.StatusBadRequest, fmt.Errorf(string(constants.ErrFailedReadRequestBody), err))
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

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

	challenge, err := webauthnhelper.ValidateAndGetChallenge(user.ID)
	if err != nil {
		shared.HandleError(w, err, http.StatusUnauthorized, err.Error())
		return
	}

	credentials := webauthnhelper.ConvertPasskeysToCredentials(passkeys)
	webAuthnUser := webauthnhelper.CreateUser(user.ID, user.Username, user.Fullname, credentials)
	credential, err := webauthnhelper.VerifyCredential(challenge, webAuthnUser, credentials, passkeys, r)
	if err != nil {
		shared.SendErrorResponse(w, shared.GetStatusCodeForWebAuthnError(err, http.StatusUnauthorized), err)
		return
	}

	capturedUserID, capturedCredID, capturedPhase := user.ID, credential.ID, user.Status.Phase
	auth.Dispatch(func() {
		_ = auth.UpdatePasskeyLastUsed(capturedUserID, capturedCredID)
		auth.UpdateUserLastLogin(capturedUserID, capturedPhase)
		webauthnhelper.CleanupChallenge(capturedUserID)
	})

	sessionToken, err := auth.CreateUserSession(user.ID, &req.DeviceMetadata)
	if err != nil {
		shared.HandleError(w, errors.New(string(constants.ErrInternalServerError)),
			http.StatusInternalServerError, err.Error())
		return
	}

	shared.SendJSONResponse(w, http.StatusOK, LoginFinishResponse{
		SessionToken: sessionToken,
		User:         user,
	})
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
	if err := shared.ValidateEmail(req.Email); err != nil {
		return nil, err
	}
	return &req, nil
}
