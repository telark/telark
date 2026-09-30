package passkey

import (
	"errors"
	"fmt"
	"net/http"
	"slices"

	authdata "github.com/telark/data/auth"
	dataerrors "github.com/telark/data/errors"
	"github.com/telark/exporter/internal/constants"
	authutils "github.com/telark/exporter/internal/utils/auth/shared"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/rest/response"
	requestutils "github.com/telark/rest/utils/request"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ExtractPasskeySpec(body map[string]any, userID string) (*authdata.Passkey, string, error) {
	passkey, err := sharedutils.ExtractStructFromBody[authdata.Passkey](body)
	if err != nil {
		return nil, constants.EmptyString, err
	}

	if err := validatePasskeyFields(passkey, userID); err != nil {
		return nil, constants.EmptyString, err
	}

	passkeyName, err := authutils.GenerateCRDName(userID, constants.ResourceTypePasskey, FindPasskeysByUserID)
	if err != nil {
		return nil, constants.EmptyString, fmt.Errorf(string(constants.ErrFailedToGenerateResourceName), err)
	}

	return passkey, passkeyName, nil
}

func validatePasskeyFields(passkey *authdata.Passkey, userID string) error {
	if err := validateRequiredPasskeyFields(passkey); err != nil {
		return err
	}

	if err := validateDeviceType(passkey.DeviceType); err != nil {
		return err
	}

	if err := checkCredentialIDUniqueness(passkey.CredentialID, userID); err != nil {
		return err
	}

	passkey.UserID = userID

	return nil
}

func validateRequiredPasskeyFields(passkey *authdata.Passkey) error {
	if err := sharedutils.ValidateRequiredField(passkey.CredentialID, string(constants.ErrPasskeyFieldRequired)); err != nil {
		return err
	}
	if err := sharedutils.ValidateRequiredField(passkey.PublicKey, string(constants.ErrPasskeyFieldRequired)); err != nil {
		return err
	}
	if err := sharedutils.ValidateRequiredField(passkey.DeviceName, string(constants.ErrPasskeyFieldRequired)); err != nil {
		return err
	}
	return sharedutils.ValidateRequiredField(passkey.DeviceType, string(constants.ErrPasskeyFieldRequired))
}

func validateDeviceType(deviceType string) error {
	if deviceType != constants.PasskeyDeviceTypePlatform &&
		deviceType != constants.PasskeyDeviceTypeCrossPlatform {
		return fmt.Errorf(string(constants.ErrPasskeyInvalidDeviceType), deviceType)
	}

	return nil
}

func checkCredentialIDUniqueness(credentialID string, userID string) error {
	// Credential IDs are stored as unpadded base64url, so the comparison is case-sensitive.
	passkeys, err := FindPasskeysByUserID(userID)
	if err != nil {
		return fmt.Errorf(string(constants.ErrFailedToListResources), constants.ResourceTypePasskey, err)
	}

	if slices.ContainsFunc(passkeys, func(existing unstructured.Unstructured) bool {
		return isDuplicateCredentialID(existing, credentialID)
	}) {
		return errors.New(string(constants.ErrPasskeyCredentialIDAlreadyExists))
	}

	return nil
}

func isDuplicateCredentialID(existingPasskey unstructured.Unstructured, credentialID string) bool {
	spec, exists := existingPasskey.Object[constants.SpecField].(map[string]any)
	if !exists {
		return false
	}

	existingCredentialID, ok := spec[constants.FieldCredentialID].(string)
	if !ok {
		return false
	}

	return existingCredentialID == credentialID
}

func ExtractPatchFields(body map[string]any) (map[string]any, error) {
	filteredPatchData := make(map[string]any)

	if deviceName, ok := body["deviceName"].(string); ok {
		filteredPatchData["deviceName"] = deviceName
	}

	if lastUsedTimestamp, ok := body["lastUsedTimestamp"].(string); ok {
		filteredPatchData["lastUsedTimestamp"] = lastUsedTimestamp
	}

	if len(body) > len(filteredPatchData) {
		return nil, errors.New(string(constants.ErrPasskeyPatchOnlyAllowedFields))
	}

	return filteredPatchData, nil
}

func ExtractPasskeyRequestParams(w http.ResponseWriter, r *http.Request) (
	userID string, credentialID string, body map[string]any, ok bool,
) {
	var err error
	userID, err = sharedutils.GetHeader(w, r, constants.HeaderUserID)
	if err != nil {
		return constants.EmptyString, constants.EmptyString, nil, false
	}

	credentialID, err = sharedutils.GetPathParam(w, r, constants.CredentialIDParam)
	if err != nil {
		return constants.EmptyString, constants.EmptyString, nil, false
	}

	body, err = requestutils.ParseRequestBody(r)
	if err != nil {
		sharedutils.LogByStatusAndSend(
			w,
			http.StatusUnprocessableEntity,
			response.OperationUnprocessed,
			fmt.Sprintf(string(dataerrors.ErrRestParseRequestBody), err),
			nil,
			err,
		)
		return constants.EmptyString, constants.EmptyString, nil, false
	}

	return userID, credentialID, body, true
}
