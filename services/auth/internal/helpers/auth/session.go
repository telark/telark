package auth

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	authdata "github.com/telark/telark/internal/data/auth"
	dataerrors "github.com/telark/telark/internal/data/errors"
	userresource "github.com/telark/telark/internal/data/resources/user"
	restshared "github.com/telark/telark/internal/rest/clients/shared"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/auth/internal/clients"
	"github.com/telark/telark/services/auth/internal/constants"
	"github.com/telark/telark/services/auth/internal/helpers/shared"
)

func ValidateSession(sessionToken string) (string, error) {
	if sessionToken == constants.EmptyString {
		return constants.EmptyString, errors.New(string(constants.ErrMissingSessionToken))
	}

	client := clients.GetSessionClient()
	session, err := client.GetSessionByToken(sessionToken)
	if err != nil {
		if errors.Is(err, restshared.ErrNotFound) {
			return constants.EmptyString, errors.New(string(constants.ErrSessionNotFound))
		}
		if errors.Is(err, restshared.ErrGone) {
			return constants.EmptyString, errors.New(string(constants.ErrSessionExpired))
		}
		lg.Warn(fmt.Sprintf(string(constants.ErrFailedGetSession), err))
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
	credID, err := shared.GetPathParam(r, constants.CredentialIDPathParam)
	if err != nil || credID == constants.EmptyString {
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

// No phase: the exporter keeps the stored one, so a stamp can never undo a
// suspension or activation an admin made while the login was in flight.
func UpdateUserLastLogin(userID string) {
	userClient := clients.GetUserClient()
	updateData := map[string]any{
		constants.UserFieldStatus: map[string]any{
			constants.UserStatusFieldLastLoginAt: time.Now().UTC().Format(constants.TimeFormatRFC3339),
		},
	}
	resp := userClient.PatchUserByID(userID, updateData)
	if resp.Status >= constants.HTTPBadRequest {
		lg.Error(fmt.Sprintf(string(constants.ErrFailedUpdateLastLogin), shared.IdentityHash(userID), resp.Status, resp.Message))
	}
}

// Every login path ends here, so this is where a terminating (410) or
// non-active account is refused a session.
func CreateUserSession(userID string, meta *authdata.DeviceMetadata) (token string, expiresAt time.Time, err error) {
	user, err := GetUserByIDWithErrorHandling(userID)
	if err != nil {
		return constants.EmptyString, time.Time{}, err
	}
	phase := userresource.AccountPhase(user.Status.Phase)
	// Callers get here only with a verified credential, so naming the suspension leaks nothing.
	if phase == userresource.AccountPhaseSuspended {
		return constants.EmptyString, time.Time{},
			shared.Refuse(constants.RefusalAccountSuspended, errors.New(string(constants.ErrUserSuspended)))
	}
	if phase != userresource.AccountPhaseActive {
		return constants.EmptyString, time.Time{},
			shared.Refuse(constants.RefusalAccountNotActive, errors.New(string(dataerrors.ErrAuthzUserNotActive)))
	}

	sessionClient := clients.GetSessionClient()
	cfg, err := shared.GetCachedConfig()
	if err != nil {
		return constants.EmptyString, time.Time{}, err
	}

	sessionToken, err := shared.GenerateSessionToken()
	if err != nil {
		return constants.EmptyString, time.Time{}, fmt.Errorf(string(constants.ErrFailedGenerateSessionToken), err)
	}

	expiresAt = time.Now().UTC().Add(time.Duration(cfg.WebAuthn.SessionExpiry) * time.Hour)

	session := &authdata.Session{
		SessionToken:     sessionToken,
		UserID:           userID,
		CreatedTimestamp: time.Now().UTC().Format(constants.TimeFormatRFC3339),
		ExpiresTimestamp: expiresAt.Format(constants.TimeFormatRFC3339),
	}

	if meta != nil {
		session.DeviceMetadata = *meta
	}

	// Checked again right before the write: a sign-in that began before break-glass froze the account sends no session
	// after it, so none can land later than the bound break-glass waits out.
	if err := RefuseFrozen(userID); err != nil {
		return constants.EmptyString, time.Time{}, err
	}
	resp := sessionClient.CreateSessionByUser(userID, session)
	if resp.Status >= constants.HTTPBadRequest {
		return constants.EmptyString, time.Time{}, fmt.Errorf(string(constants.ErrFailedCreateSession), resp.Message)
	}

	return sessionToken, expiresAt, nil
}

// The session leaves auth only in this cookie: HttpOnly, so no script reads it, Strict and host-only, and Secure,
// since auth can't tell the browser's scheme behind nginx (browsers but Safari keep it on http://localhost). It
// ends with the session, and is __Host- prefixed when every passkey origin is https (xauthz.SessionCookieName).
func SetSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, sessionCookie(token, max(int(time.Until(expiresAt).Seconds()), constants.DefaultIncrementValue)))
}

func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, sessionCookie(constants.EmptyString, constants.CookieMaxAgeDelete))
}

func sessionCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     xauthz.SessionCookieName(),
		Value:    value,
		Path:     constants.CookiePath,
		MaxAge:   maxAge,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
}
