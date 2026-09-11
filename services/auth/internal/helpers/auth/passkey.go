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
	"github.com/telark/auth/internal/constants"
	"github.com/telark/auth/internal/helpers/shared"
	authdata "github.com/telark/data/auth"
	userresource "github.com/telark/data/resources/user"
)

// ConflictError represents a 409 Conflict error from the data service
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
		constants.PasskeyFieldLastUsedTimestamp: time.Now().Format(constants.TimeFormatRFC3339),
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

func CheckUserHasExistingPasskeys(userID string) (bool, error) {
	passkeyClient := clients.GetPasskeyClient()
	passkeys, err := passkeyClient.GetAllPasskeysByUser(userID)
	if err != nil {
		return false, err
	}
	return len(passkeys) > constants.InitialCapacity && passkeys[constants.DefaultInitValue] != nil, nil
}

func GetUserForRegistrationStart(r *http.Request) (*userresource.UserAsResource, string, error) {
	userClient := clients.GetUserClient()
	sessionUserID, sessionErr := ValidateSessionFromRequest(r)
	if sessionErr == nil {
		user, err := GetUserWithErrorHandling(sessionUserID, userClient.GetUserByID)
		if err != nil {
			return nil, constants.EmptyString, err
		}
		return user, sessionUserID, nil
	}

	var req struct {
		Email string `json:"email,omitempty"`
	}
	if err := shared.DecodeRequestBody(r, &req); err != nil {
		return nil, constants.EmptyString, err
	}

	if err := shared.ValidateEmail(req.Email); err != nil {
		return nil, constants.EmptyString, err
	}

	user, err := GetUserWithErrorHandling(req.Email, userClient.GetUserByEmail)
	if err != nil {
		if !shared.IsError(err, constants.ErrUserNotFound) {
			return nil, constants.EmptyString, err
		}
		user, err = JitProvisionUserByEmail(userClient, req.Email)
		if err != nil {
			return nil, constants.EmptyString, err
		}
		return user, user.ID, nil
	}

	hasPasskeys, err := CheckUserHasExistingPasskeys(user.ID)
	if err == nil && hasPasskeys {
		return nil, constants.EmptyString, errors.New(string(constants.ErrUserAlreadyHasPasskeysPleaseLoginFirst))
	}

	return user, user.ID, nil
}

func GetUserForRegistration(r *http.Request) (*userresource.UserAsResource, string, error) {
	userClient := clients.GetUserClient()
	sessionUserID, sessionErr := ValidateSessionFromRequest(r)
	if sessionErr == nil {
		user, err := GetUserWithErrorHandling(sessionUserID, userClient.GetUserByID)
		if err != nil {
			return nil, constants.EmptyString, err
		}
		return user, sessionUserID, nil
	}

	email := r.Header.Get(constants.HeaderEmail)
	if email == constants.EmptyString {
		return nil, constants.EmptyString,
			errors.New(string(constants.ErrEmailRequiredForUnauthenticatedRegistration))
	}

	if err := shared.ValidateEmail(email); err != nil {
		return nil, constants.EmptyString, err
	}

	user, err := GetUserWithErrorHandling(email, userClient.GetUserByEmail)
	if err != nil {
		if !shared.IsError(err, constants.ErrUserNotFound) {
			return nil, constants.EmptyString, err
		}
		user, err = JitProvisionUserByEmail(userClient, email)
		if err != nil {
			return nil, constants.EmptyString, err
		}
		return user, user.ID, nil
	}

	hasPasskeys, err := CheckUserHasExistingPasskeys(user.ID)
	if err == nil && hasPasskeys {
		return nil, constants.EmptyString, errors.New(string(constants.ErrUserAlreadyHasPasskeys))
	}

	return user, user.ID, nil
}

func AttachPasskeyIdentity(
	userID string,
	user *userresource.UserAsResource,
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
) *authdata.UserPasskey {
	credIDStr := base64.RawURLEncoding.EncodeToString(credential.ID)
	pubKeyStr := base64.StdEncoding.EncodeToString(credential.PublicKey)
	now := time.Now()
	creationTime := now.Format(constants.TimeFormatRFC3339)
	lastUsedTime := now.Format(constants.TimeFormatRFC3339)
	return &authdata.UserPasskey{
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

func CreatePasskey(userID string, passkey *authdata.UserPasskey) (any, error) {
	passkeyClient := clients.GetPasskeyClient()
	resp := passkeyClient.CreatePasskeyByUser(userID, passkey)
	if resp.Status >= constants.HTTPBadRequest {
		// Preserve 409 Conflict status
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
