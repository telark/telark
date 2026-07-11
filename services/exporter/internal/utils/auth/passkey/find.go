package passkey

import (
	"errors"
	"net/http"

	authmetadata "github.com/telark/data/metadata/auth"
	"github.com/telark/exporter/constants"
	authshared "github.com/telark/exporter/utils/auth/shared"
	sharedutils "github.com/telark/exporter/utils/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func FindPasskeyByCredentialID(credentialID string) (*unstructured.Unstructured, error) {
	return sharedutils.FindResourceBySpecField(
		authmetadata.UserPasskeyMetadata,
		constants.ErrPasskeyListFormatInvalid,
		constants.FieldCredentialID,
		credentialID,
		constants.ErrPasskeyNotFound,
	)
}

func FindPasskeysByUserID(userID string) ([]unstructured.Unstructured, error) {
	return authshared.FindResourcesByUserID(
		authmetadata.UserPasskeyMetadata,
		constants.ErrPasskeyListFormatInvalid,
		userID,
	)
}

func FindPasskeyOrRespond(w http.ResponseWriter, credentialID string) (*unstructured.Unstructured, bool) {
	return authshared.FindResourceOrRespond(
		w,
		func() (*unstructured.Unstructured, error) {
			return FindPasskeyByCredentialID(credentialID)
		},
		constants.ErrPasskeyNotFoundForCredentialID,
		credentialID,
	)
}

func FindPasskeyByCredentialIDAndUserID(credentialID string, userID string) (*unstructured.Unstructured, error) {
	passkey, err := FindPasskeyByCredentialID(credentialID)
	if err != nil {
		return nil, err
	}

	// Verify the passkey belongs to the user
	if spec, exists := passkey.Object[constants.SpecField].(map[string]any); exists {
		if passkeyUserID, ok := spec[constants.UserIDParam].(string); ok && passkeyUserID == userID {
			return passkey, nil
		}
	}

	return nil, errors.New(string(constants.ErrPasskeyNotFoundForUser))
}
