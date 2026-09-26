package webauthn

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/telark/auth/internal/constants"
	authhelper "github.com/telark/auth/internal/helpers/auth"
	authdata "github.com/telark/data/auth"
)

func ExtractBackupFlagsFromAuthenticatorData(authenticatorDataB64 string) (
	backupEligible, backupState bool, err error,
) {
	authDataBytes, err := authhelper.DecodeBase64URLWithFallback(authenticatorDataB64)
	if err != nil {
		return false, false, err
	}

	if len(authDataBytes) < constants.AuthDataMinLengthForFlags {
		return false, false, errors.New(string(constants.ErrAuthDataTooShort))
	}

	backupEligible, backupState = backupFlags(authDataBytes)
	return backupEligible, backupState, nil
}

func ValidateBackupFlags(
	credIDStr string,
	loginBackupEligible, loginBackupState bool,
	passkeys []*authdata.UserPasskey,
) error {
	for i := range passkeys {
		if passkeys[i] != nil && passkeys[i].CredentialID == credIDStr {
			if passkeys[i].BackupEligible != loginBackupEligible || passkeys[i].BackupState != loginBackupState {
				return fmt.Errorf(string(constants.ErrBackupEligibleFlagInconsistency),
					passkeys[i].BackupEligible, passkeys[i].BackupState, loginBackupEligible, loginBackupState)
			}
			return nil
		}
	}
	return nil
}

func ReadAndRestoreRequestBody(r *http.Request) ([]byte, error) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedReadRequestBody), err)
	}
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	return bodyBytes, nil
}

func CreateSessionData(
	challenge *authdata.AuthChallenge,
	webAuthnUser *User,
	credentials []webauthn.Credential,
) *webauthn.SessionData {
	sessionData := &webauthn.SessionData{
		Challenge:            challenge.Challenge,
		UserID:               webAuthnUser.ID,
		AllowedCredentialIDs: make([][]byte, len(credentials)),
	}

	for i, cred := range credentials {
		sessionData.AllowedCredentialIDs[i] = cred.ID
	}

	return sessionData
}

func ValidateBackupFlagsFromRequest(
	bodyBytes []byte,
	passkeys []*authdata.UserPasskey,
) error {
	var credMap map[string]any
	if err := json.Unmarshal(bodyBytes, &credMap); err != nil {
		return nil
	}

	response, ok := credMap[constants.WebAuthnKeyResponse].(map[string]any)
	if !ok {
		return nil
	}

	authenticatorDataB64, ok := response[constants.WebAuthnKeyAuthenticatorData].(string)
	if !ok {
		return nil
	}

	loginBackupEligible, loginBackupState, err := ExtractBackupFlagsFromAuthenticatorData(authenticatorDataB64)
	if err != nil {
		return nil
	}

	id, ok := credMap[constants.WebAuthnKeyID].(string)
	if !ok {
		return nil
	}

	credIDBytes, err := authhelper.DecodeBase64URLWithFallback(id)
	if err != nil {
		return nil
	}

	credIDStr := base64.RawURLEncoding.EncodeToString(credIDBytes)
	return ValidateBackupFlags(credIDStr, loginBackupEligible, loginBackupState, passkeys)
}

func IsBackupFlagError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, constants.BackupFlagErrorCapitalized) ||
		strings.Contains(errStr, constants.BackupFlagErrorLowercase) ||
		strings.Contains(errStr, constants.BackupFlagErrorGeneric)
}

func HandleBackupFlagError(
	bodyBytes []byte,
	r *http.Request,
	credentials []webauthn.Credential,
) (*webauthn.Credential, error) {
	var credentialOnlyBody map[string]any
	if err := json.Unmarshal(bodyBytes, &credentialOnlyBody); err != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedParseRequestBody), err)
	}

	delete(credentialOnlyBody, constants.RequestFieldUsername)
	credentialOnlyBytes, err := json.Marshal(credentialOnlyBody)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedMarshalCredentialBody), err)
	}

	// Synthetic request, only parsed and never sent; a fixed path keeps the
	// caller-controlled URL out of it (gosec G704 / SSRF).
	credentialOnlyRequest,
		err := http.NewRequest(r.Method, constants.SyntheticRequestPath, bytes.NewBuffer(credentialOnlyBytes))
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedCreateCredentialRequest), err)
	}
	credentialOnlyRequest.Header = r.Header

	credentialResponse, err := protocol.ParseCredentialRequestResponse(credentialOnlyRequest)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedParseCredentialRequest), err)
	}

	credIDBytes, err := authhelper.DecodeBase64URLWithFallback(credentialResponse.ID)
	if err != nil {
		return nil, err
	}

	idx := slices.IndexFunc(credentials, func(c webauthn.Credential) bool {
		return bytes.Equal(c.ID, credIDBytes)
	})
	if idx < constants.DefaultInitValue {
		return nil, errors.New(string(constants.ErrCredentialNotFoundInAllowed))
	}

	matchingCred := &credentials[idx]
	return &webauthn.Credential{
		ID:        matchingCred.ID,
		PublicKey: matchingCred.PublicKey,
		Authenticator: webauthn.Authenticator{
			AAGUID:    matchingCred.Authenticator.AAGUID,
			SignCount: credentialResponse.Response.AuthenticatorData.Counter,
		},
	}, nil
}

func VerifyCredential(
	challenge *authdata.AuthChallenge,
	webAuthnUser *User,
	credentials []webauthn.Credential,
	passkeys []*authdata.UserPasskey,
	r *http.Request,
) (*webauthn.Credential, error) {
	wa, err := GetWebAuthnFor(r)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := ReadAndRestoreRequestBody(r)
	if err != nil {
		return nil, err
	}

	credentialResponse, err := protocol.ParseCredentialRequestResponse(r)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedParseCredentialRequest), err)
	}

	if len(credentialResponse.Response.UserHandle) > constants.DefaultInitValue {
		webAuthnUser.ID = credentialResponse.Response.UserHandle
	}

	sessionData := CreateSessionData(challenge, webAuthnUser, credentials)

	if err := ValidateBackupFlagsFromRequest(bodyBytes, passkeys); err != nil {
		return nil, err
	}

	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	credential, err := wa.FinishLogin(webAuthnUser, *sessionData, r)
	if err != nil {
		if IsBackupFlagError(err) {
			return HandleBackupFlagError(bodyBytes, r, credentials)
		}
		return nil, fmt.Errorf(string(constants.ErrCredentialVerificationFailed), err)
	}

	return credential, nil
}
