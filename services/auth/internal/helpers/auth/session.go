package auth

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/telark/auth/internal/clients"
	"github.com/telark/auth/internal/constants"
	"github.com/telark/auth/internal/helpers/shared"
	authdata "github.com/telark/data/auth"
	restshared "github.com/telark/rest/clients/shared"
)

func ValidateSession(sessionToken string) (string, error) {
	if sessionToken == constants.EmptyString {
		return constants.EmptyString, errors.New(string(constants.ErrMissingSessionToken))
	}

	client := clients.GetSessionClient()
	session, err := client.GetSessionByToken(sessionToken)
	if err != nil {
		lg.Warn(fmt.Sprintf(string(constants.ErrFailedGetSession), err))
		if errors.Is(err, restshared.ErrNotFound) {
			return constants.EmptyString, errors.New(string(constants.ErrSessionNotFound))
		}
		return constants.EmptyString, shared.ErrBackendUnavailable
	}

	if session == nil {
		return constants.EmptyString, errors.New(string(constants.ErrSessionNotFound))
	}

	expiresAt, err := time.Parse(constants.TimeFormatRFC3339, session.ExpiresTimestamp)
	if err != nil || time.Now().After(expiresAt) {
		return constants.EmptyString, errors.New(string(constants.ErrSessionExpired))
	}

	return session.UserID, nil
}

func ExtractSessionToken(r *http.Request) (string, error) {
	token := r.Header.Get(constants.HeaderSessionToken)
	if token == constants.EmptyString {
		return constants.EmptyString, errors.New(string(constants.ErrMissingSessionToken))
	}
	return token, nil
}

func ExtractCredentialID(r *http.Request) (string, error) {
	credID := r.Header.Get(constants.HeaderCredentialID)
	if credID == constants.EmptyString {
		return constants.EmptyString, errors.New(string(constants.ErrMissingCredentialID))
	}
	return credID, nil
}

func ValidateSessionFromRequest(r *http.Request) (string, error) {
	sessionToken, err := ExtractSessionToken(r)
	if err != nil {
		return constants.EmptyString, err
	}
	return ValidateSession(sessionToken)
}

func ValidateSessionAndExtractCredentialID(r *http.Request) (userID string, credentialID string, err error) {
	userID, err = ValidateSessionFromRequest(r)
	if err != nil {
		return constants.EmptyString, constants.EmptyString, err
	}
	credentialID, err = ExtractCredentialID(r)
	if err != nil {
		return constants.EmptyString, constants.EmptyString, err
	}
	return userID, credentialID, nil
}

// Phase is passed in rather than re-fetched: a status patch replaces the whole
// field, so omitting it would silently reset the account back to active.
func UpdateUserLastLogin(userID, phase string) {
	userClient := clients.GetUserClient()
	updateData := map[string]any{
		constants.UserFieldStatus: map[string]any{
			constants.UserStatusFieldPhase:       phase,
			constants.UserStatusFieldLastLoginAt: time.Now().UTC().Format(constants.TimeFormatRFC3339),
		},
	}
	resp := userClient.PatchUserByID(userID, updateData)
	if resp.Status >= constants.HTTPBadRequest {
		lg.Error(fmt.Sprintf(string(constants.ErrFailedUpdateLastLogin), shared.IdentityHash(userID), resp.Status, resp.Message))
	}
}

func CreateUserSession(userID string, meta *authdata.DeviceMetadata) (string, error) {
	sessionClient := clients.GetSessionClient()
	cfg, err := shared.GetCachedConfig()
	if err != nil {
		return constants.EmptyString, err
	}

	sessionToken, err := shared.GenerateSessionToken()
	if err != nil {
		return constants.EmptyString, fmt.Errorf(string(constants.ErrFailedGenerateSessionToken), err)
	}

	expiresAt := time.Now().UTC().Add(time.Duration(cfg.WebAuthn.SessionExpiry) * time.Hour)

	session := &authdata.UserSession{
		SessionToken:     sessionToken,
		UserID:           userID,
		CreatedTimestamp: time.Now().UTC().Format(constants.TimeFormatRFC3339),
		ExpiresTimestamp: expiresAt.Format(constants.TimeFormatRFC3339),
	}

	if meta != nil {
		session.DeviceMetadata = *meta
	}

	resp := sessionClient.CreateSessionByUser(userID, session)
	if resp.Status >= constants.HTTPBadRequest {
		return constants.EmptyString, fmt.Errorf(string(constants.ErrFailedCreateSession), resp.Message)
	}

	return sessionToken, nil
}
