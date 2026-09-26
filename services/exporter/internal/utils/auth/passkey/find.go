package passkey

import (
	"errors"

	authmetadata "github.com/telark/data/metadata/v1alpha1"
	"github.com/telark/exporter/internal/constants"
	authshared "github.com/telark/exporter/internal/utils/auth/shared"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func FindPasskeyByCredentialID(credentialID string) (*unstructured.Unstructured, error) {
	return sharedutils.FindResourceBySpecField(
		authmetadata.PasskeyMetadata,
		constants.ErrPasskeyListFormatInvalid,
		constants.FieldCredentialID,
		credentialID,
		constants.ErrPasskeyNotFound,
	)
}

func FindPasskeysByUserID(userID string) ([]unstructured.Unstructured, error) {
	return authshared.FindResourcesByUserID(
		authmetadata.PasskeyMetadata,
		constants.ErrPasskeyListFormatInvalid,
		userID,
	)
}

func FindPasskeyByCredentialIDAndUserID(credentialID string, userID string) (*unstructured.Unstructured, error) {
	passkey, err := FindPasskeyByCredentialID(credentialID)
	if err != nil {
		return nil, err
	}

	if spec, exists := passkey.Object[constants.SpecField].(map[string]any); exists {
		if passkeyUserID, ok := spec[constants.UserIDParam].(string); ok && passkeyUserID == userID {
			return passkey, nil
		}
	}

	return nil, errors.New(string(constants.ErrPasskeyNotFoundForUser))
}
