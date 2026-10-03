package auth

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	webauthnlib "github.com/go-webauthn/webauthn/webauthn"
	authdata "github.com/telark/telark/internal/data/auth"
	dataerrors "github.com/telark/telark/internal/data/errors"
	userresource "github.com/telark/telark/internal/data/resources/user"
	"github.com/telark/telark/services/auth/internal/clients"
	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	"github.com/telark/telark/services/auth/internal/helpers/shared"
)

// The exporter's own status and message, so a handler can relay a refusal
// (not found, last passkey, conflict) instead of flattening it into a 500.
type ProxyError struct {
	Status  int
	Message string
}

func (e *ProxyError) Error() string {
	return e.Message
}

func proxyError(status int, wrapped string) error {
	return &ProxyError{Status: status, Message: shared.ExporterMessage(wrapped)}
}

func UpdatePasskeyLastUsed(userID string, credentialID []byte) error {
	passkeyClient := clients.GetPasskeyClient()
	credIDStr := base64.RawURLEncoding.EncodeToString(credentialID)
	updateData := map[string]any{
		constants.PasskeyFieldLastUsedTimestamp: time.Now().UTC().Format(constants.TimeFormatRFC3339),
	}

	resp := passkeyClient.PatchPasskeyByUserAndCredentialID(userID, credIDStr, updateData)
	if resp.Status >= constants.HTTPBadRequest {
		lg.Error(fmt.Sprintf(string(constants.ErrFailedUpdatePasskey), resp.Message))
		return fmt.Errorf(string(constants.ErrFailedUpdatePasskey), resp.Message)
	}
	return nil
}

func DecodeBase64URLWithFallback(encoded string) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err == nil {
		return decoded, nil
	}

	const (
		base64PaddingModulo = 4
	)
	base64Str := strings.ReplaceAll(encoded, constants.Base64URLMinus, constants.Base64URLPlus)
	base64Str = strings.ReplaceAll(base64Str, constants.Base64URLUnderscore, constants.Base64URLSlash)
	for len(base64Str)%base64PaddingModulo != constants.DefaultInitValue {
		base64Str += constants.Base64URLPadding
	}
	return base64.StdEncoding.DecodeString(base64Str)
}

// A lookup that could not answer counts as "has passkeys": a session-less
// registration must never succeed because the backend was unreachable.
func CheckUserHasExistingPasskeys(userID string) (bool, error) {
	passkeyClient := clients.GetPasskeyClient()
	passkeys, err := passkeyClient.GetAllPasskeysByUser(userID)
	if err != nil {
		return true, err
	}
	return len(passkeys) > constants.InitialCapacity && passkeys[constants.DefaultInitValue] != nil, nil
}

func registerStartBody(r *http.Request) (email, enrollToken string, err error) {
	if r.ContentLength == constants.DefaultInitValue {
		return constants.EmptyString, constants.EmptyString, nil
	}
	var req struct {
		Email       string `json:"email,omitempty"`
		EnrollToken string `json:"enrollToken,omitempty"`
	}
	if err := shared.DecodeRequestBody(r, &req); err != nil {
		return constants.EmptyString, constants.EmptyString, err
	}
	return req.Email, req.EnrollToken, nil
}

// A caller who proved an identity (session or enrollment token) registers for
// that user: the body may repeat its email or omit it, never name another account.
func ownUser(userID, email string) (*userresource.User, error) {
	user, err := GetUserByIDWithErrorHandling(userID)
	if err != nil {
		return nil, err
	}
	if email != constants.EmptyString && !strings.EqualFold(email, user.Email) {
		return nil, errors.New(string(constants.ErrRegisterEmailMismatch))
	}
	return user, nil
}

// A bare email proves nothing, so it may only open a brand-new account: an
// existing one needs a session or an enrollment token, and a bootstrap email is
// enrolled by the operator (break-glass), never by whoever claims it first.
func userForEmail(email string) (*userresource.User, string, error) {
	if err := shared.ValidateEmail(email); err != nil {
		return nil, constants.EmptyString, err
	}
	if config.IsBootstrapAdmin(email) {
		return nil, constants.EmptyString, errors.New(string(constants.ErrReservedEmail))
	}

	userClient := clients.GetUserClient()
	_, err := GetUserWithErrorHandling(email, userClient.GetUserByEmail)
	if err == nil {
		return nil, constants.EmptyString, errors.New(string(constants.ErrRegistrationNeedsProof))
	}
	if !shared.IsError(err, constants.ErrUserNotFound) {
		return nil, constants.EmptyString, err
	}
	return storePendingUser(email)
}

// Strongest proof wins: session, then one-time enrollment token (reported as
// enrolled so the session-less finish may add to an account with passkeys), then email.
func GetUserForRegistrationStart(r *http.Request) (
	user *userresource.User, userID string, enrolled bool, err error,
) {
	sessionUserID, sessionErr := ValidateSessionFromRequest(r)
	email, enrollToken, err := registerStartBody(r)
	if err != nil {
		return nil, constants.EmptyString, false, err
	}

	if sessionErr == nil {
		user, err = ownUser(sessionUserID, email)
		if err != nil {
			return nil, constants.EmptyString, false, err
		}
		return user, sessionUserID, false, nil
	}

	if enrollToken != constants.EmptyString {
		userID, err = ResolveEnrollToken(enrollToken)
		if err != nil {
			return nil, constants.EmptyString, false, err
		}
		user, err = ownUser(userID, email)
		// A link that outlived its user answers like any other dead link, so it reveals no deletion.
		if shared.IsError(err, constants.ErrUserNotFound) || shared.IsError(err, dataerrors.ErrAuthzUserNotActive) {
			return nil, constants.EmptyString, false, errors.New(string(constants.ErrEnrollTokenInvalid))
		}
		if err != nil {
			return nil, constants.EmptyString, false, err
		}
		return user, userID, true, nil
	}

	user, userID, err = userForEmail(email)
	if err != nil {
		return nil, constants.EmptyString, false, err
	}
	return user, userID, false, nil
}

// The identity headers are stripped by the authz layer, so for a session-less
// finish only the signed ceremony (ceremonyOwner) can name the user.
func GetUserForRegistration(
	r *http.Request, ceremonyOwner func(*http.Request) (string, bool, error),
) (user *userresource.User, userID string, enrolled bool, err error) {
	sessionUserID, sessionErr := ValidateSessionFromRequest(r)
	if sessionErr == nil {
		user, err = GetUserByIDWithErrorHandling(sessionUserID)
		if err != nil {
			return nil, constants.EmptyString, false, err
		}
		return user, sessionUserID, false, nil
	}

	ownerID, enrolled, err := ceremonyOwner(r)
	if err != nil {
		return nil, constants.EmptyString, false, err
	}
	pending, found, err := pendingUser(ownerID)
	if err != nil {
		return nil, constants.EmptyString, false, err
	}
	if found {
		return pending, ownerID, false, nil
	}

	user, err = GetUserByIDWithErrorHandling(ownerID)
	if err != nil {
		return nil, constants.EmptyString, false, err
	}
	if enrolled {
		return user, user.ID, true, nil
	}

	hasPasskeys, _ := CheckUserHasExistingPasskeys(user.ID)
	if hasPasskeys {
		return nil, constants.EmptyString, false, errors.New(string(constants.ErrUserAlreadyHasPasskeys))
	}

	return user, user.ID, false, nil
}

func AttachPasskeyIdentity(
	userID string,
	user *userresource.User,
	credential *webauthnlib.Credential,
) error {
	credIDStr := base64.RawURLEncoding.EncodeToString(credential.ID)
	identity := &userresource.UserIdentity{
		Provider: constants.IdentityProviderPasskey,
		Issuer:   constants.EmptyString,
		Subject:  credIDStr,
	}
	user.Identities = append(user.Identities, identity)
	userClient := clients.GetUserClient()
	resp := userClient.PatchUserByID(userID, map[string]any{constants.UserFieldIdentities: user.Identities})
	if resp.Status != http.StatusOK {
		return fmt.Errorf(string(constants.ErrFailedAttachIdentity), shared.IdentityHash(userID), resp.Status)
	}
	return nil
}

func ValidateDeviceHeaders(r *http.Request) (deviceName, deviceType string, err error) {
	deviceName = r.Header.Get(constants.HeaderDeviceName)
	deviceType = r.Header.Get(constants.HeaderDeviceType)
	if deviceName == constants.EmptyString || deviceType == constants.EmptyString {
		return constants.EmptyString, constants.EmptyString, errors.New(string(constants.ErrMissingRequiredFields))
	}
	return deviceName, deviceType, nil
}

func CreatePasskeyFromCredential(
	userID string,
	credential *webauthnlib.Credential,
	deviceName, deviceType string,
	backupEligible, backupState bool,
) *authdata.Passkey {
	credIDStr := base64.RawURLEncoding.EncodeToString(credential.ID)
	pubKeyStr := base64.StdEncoding.EncodeToString(credential.PublicKey)
	now := time.Now().UTC()
	creationTime := now.Format(constants.TimeFormatRFC3339)
	lastUsedTime := now.Format(constants.TimeFormatRFC3339)
	return &authdata.Passkey{
		UserID:            userID,
		CredentialID:      credIDStr,
		PublicKey:         pubKeyStr,
		DeviceName:        deviceName,
		DeviceType:        deviceType,
		CreationTimestamp: &creationTime,
		LastUsedTimestamp: &lastUsedTime,
		BackupEligible:    backupEligible,
		BackupState:       backupState,
	}
}

func CreatePasskey(userID string, passkey *authdata.Passkey) (any, error) {
	passkeyClient := clients.GetPasskeyClient()
	resp := passkeyClient.CreatePasskeyByUser(userID, passkey)
	if resp.Status >= constants.HTTPBadRequest {
		return nil, proxyError(resp.Status, resp.Message)
	}
	return resp.Data, nil
}

func UpdatePasskey(userID, credentialID string, updateData map[string]any) (any, error) {
	passkeyClient := clients.GetPasskeyClient()
	resp := passkeyClient.PatchPasskeyByUserAndCredentialID(userID, credentialID, updateData)
	if resp.Status >= constants.HTTPBadRequest {
		return nil, proxyError(resp.Status, resp.Message)
	}
	return resp.Data, nil
}

// Both the user's own delete and the orphan cleanup come through here, so the
// identity the passkey was registered with is dropped in one place.
func DeletePasskey(userID, credentialID string, forceLastDelete bool) error {
	passkeyClient := clients.GetPasskeyClient()
	resp := passkeyClient.DeletePasskeyByUserAndCredentialID(userID, credentialID, forceLastDelete)
	if resp.Status >= constants.HTTPBadRequest {
		return proxyError(resp.Status, resp.Message)
	}
	detachPasskeyIdentity(userID, credentialID)
	return nil
}

// The passkey is already gone, so a failed detach is logged rather than reported:
// the stale identity only keeps a later Google sign-in from attaching by email.
func detachPasskeyIdentity(userID, credentialID string) {
	user, err := GetUserByIDWithErrorHandling(userID)
	if err != nil {
		if !shared.IsError(err, constants.ErrUserNotFound) {
			lg.Warn(fmt.Sprintf(string(constants.ErrFailedDetachIdentity), shared.IdentityHash(userID), err))
		}
		return
	}
	kept := slices.DeleteFunc(slices.Clone(user.Identities), func(identity *userresource.UserIdentity) bool {
		return identity != nil && identity.Provider == constants.IdentityProviderPasskey && identity.Subject == credentialID
	})
	if len(kept) == len(user.Identities) {
		return
	}
	resp := clients.GetUserClient().PatchUserByID(userID, map[string]any{constants.UserFieldIdentities: kept})
	if resp.Status != http.StatusOK {
		lg.Warn(fmt.Sprintf(string(constants.ErrFailedDetachIdentity), shared.IdentityHash(userID), resp.Status))
	}
}
