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
	authdata "github.com/telark/data/auth"
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
		statusCode := http.StatusInternalServerError
		if shared.IsError(err, constants.ErrUserNotFound) || shared.IsError(err, constants.ErrNoPasskeysFound) {
			statusCode = http.StatusNotFound
		}
		shared.SendErrorResponse(w, statusCode, err)
		return
	}

	credentials := webauthnhelper.ConvertPasskeysToCredentials(passkeys)
	webAuthnUser := webauthnhelper.CreateUser(user.ID, user.Username, user.Fullname, credentials)

	wa, err := webauthnhelper.GetWebAuthn()
	if err != nil {
		shared.HandleError(w, fmt.Errorf(string(constants.ErrWebAuthnSetupFailed), err),
			http.StatusInternalServerError,
			fmt.Sprintf(string(constants.ErrWebAuthnSetupFailed), err))
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
	lg.Info(string(constants.SuccessLoginStarted))
}

func LoginFinish(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		shared.SendErrorResponse(w, http.StatusBadRequest, fmt.Errorf(string(constants.ErrFailedReadRequestBody), err))
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	email, err := extractLoginEmail(bodyBytes)
	if err != nil {
		shared.SendErrorResponse(w, http.StatusBadRequest, err)
		return
	}

	user, passkeys, err := auth.GetUserAndPasskeys(email)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if shared.IsError(err, constants.ErrUserNotFound) || shared.IsError(err, constants.ErrNoPasskeysFound) {
			statusCode = http.StatusNotFound
		}
		shared.SendErrorResponse(w, statusCode, err)
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
		shared.SendErrorResponse(w, http.StatusUnauthorized, err)
		return
	}

	capturedUserID, capturedCredID := user.ID, credential.ID
	auth.Dispatch(func() {
		if err := auth.UpdatePasskeyLastUsed(capturedUserID, capturedCredID); err != nil {
			lg.Error(fmt.Sprintf(string(constants.ErrFailedUpdatePasskeyLastUsed), err))
		}
		webauthnhelper.CleanupChallenge(capturedUserID)
	})

	meta := extractDeviceMeta(bodyBytes)

	sessionToken, err := auth.CreateUserSession(user.ID, meta)
	if err != nil {
		shared.HandleError(w, errors.New(string(constants.ErrInternalServerError)),
			http.StatusInternalServerError, err.Error())
		return
	}

	shared.SendJSONResponse(w, http.StatusOK, LoginFinishResponse{
		SessionToken: sessionToken,
		User:         user,
	})
	lg.Info(string(constants.SuccessLoginCompleted))
}

func extractDeviceMeta(bodyBytes []byte) *authdata.DeviceMetadata {
	req, err := extractLoginRequest(bodyBytes)
	if err != nil {
		lg.Info(fmt.Sprintf(string(constants.LogExtractLoginRequestFailed), err))
		return nil
	}
	return &req.DeviceMetadata
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

func extractLoginEmail(bodyBytes []byte) (string, error) {
	req, err := extractLoginRequest(bodyBytes)
	if err != nil {
		return constants.EmptyString, err
	}
	return req.Email, nil
}
