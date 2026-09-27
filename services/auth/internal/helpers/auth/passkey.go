package auth

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	webauthnlib "github.com/go-webauthn/webauthn/webauthn"
	"github.com/telark/auth/internal/clients"
	"github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
	"github.com/telark/auth/internal/helpers/shared"
	authdata "github.com/telark/data/auth"
	userresource "github.com/telark/data/resources/user"
)

type ConflictError struct {
	Message string
}

func (e *ConflictError) Error() string {
	return e.Message
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
func userForEmail(email string) (*userresource.User, error) {
	if err := shared.ValidateEmail(email); err != nil {
		return nil, err
	}
	if config.IsBootstrapAdmin(email) {
		return nil, errors.New(string(constants.ErrReservedEmail))
	}

	userClient := clients.GetUserClient()
	_, err := GetUserWithErrorHandling(email, userClient.GetUserByEmail)
	if err == nil {
		return nil, errors.New(string(constants.ErrRegistrationNeedsProof))
	}
	if !shared.IsError(err, constants.ErrUserNotFound) {
		return nil, err
	}
	return JitProvisionUserByEmail(userClient, email)
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
		if err != nil {
			return nil, constants.EmptyString, false, err
		}
		return user, userID, true, nil
	}

	user, err = userForEmail(email)
	if err != nil {
		return nil, constants.EmptyString, false, err
	}
	return user, user.ID, false, nil
}

// The identity headers are stripped by the authz layer, so for a session-less
// finish only the signed ceremony (ceremonyOwner) can name the user.
func GetUserForRegistration(
	r *http.Request, ceremonyOwner func(*http.Request) (string, bool, error),
) (*userresource.User, string, error) {
	sessionUserID, sessionErr := ValidateSessionFromRequest(r)
	if sessionErr == nil {
		user, err := GetUserByIDWithErrorHandling(sessionUserID)
		if err != nil {
			return nil, constants.EmptyString, err
		}
		return user, sessionUserID, nil
	}

	ownerID, enrolled, err := ceremonyOwner(r)
	if err != nil {
		return nil, constants.EmptyString, err
	}

	user, err := GetUserByIDWithErrorHandling(ownerID)
	if err != nil {
		return nil, constants.EmptyString, err
	}
	if enrolled {
		return user, user.ID, nil
	}

	hasPasskeys, _ := CheckUserHasExistingPasskeys(user.ID)
	if hasPasskeys {
		return nil, constants.EmptyString, errors.New(string(constants.ErrUserAlreadyHasPasskeys))
	}

	return user, user.ID, nil
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
		if resp.Status == http.StatusConflict {
			return nil, &ConflictError{Message: resp.Message}
		}
		return nil, fmt.Errorf(string(constants.ErrFailedProxyRequest), resp.Message)
	}
	return resp.Data, nil
}

func UpdatePasskey(userID, credentialID string, updateData map[string]any) (any, error) {
	passkeyClient := clients.GetPasskeyClient()
	resp := passkeyClient.PatchPasskeyByUserAndCredentialID(userID, credentialID, updateData)
	if resp.Status >= constants.HTTPBadRequest {
		return nil, fmt.Errorf(string(constants.ErrFailedProxyRequest), resp.Message)
	}
	return resp.Data, nil
}

func DeletePasskey(userID, credentialID string, forceLastDelete bool) error {
	passkeyClient := clients.GetPasskeyClient()
	resp := passkeyClient.DeletePasskeyByUserAndCredentialID(userID, credentialID, forceLastDelete)
	if resp.Status >= constants.HTTPBadRequest {
		return fmt.Errorf(string(constants.ErrFailedProxyRequest), resp.Message)
	}
	return nil
}
