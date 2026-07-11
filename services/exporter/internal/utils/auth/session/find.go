package session

import (
	"net/http"

	authmetadata "github.com/telark/data/metadata/auth"
	"github.com/telark/exporter/constants"
	"github.com/telark/exporter/utils/auth/shared"
	sharedutils "github.com/telark/exporter/utils/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func FindSessionByToken(token string) (*unstructured.Unstructured, error) {
	return sharedutils.FindResourceBySpecField(
		authmetadata.UserSessionMetadata,
		constants.ErrSessionListFormatInvalid,
		constants.FieldSessionToken,
		token,
		constants.ErrSessionNotFound,
	)
}

func FindSessionsByUserID(userID string) ([]unstructured.Unstructured, error) {
	return shared.FindResourcesByUserID(
		authmetadata.UserSessionMetadata,
		constants.ErrSessionListFormatInvalid,
		userID,
	)
}

func FindSessionOrRespond(w http.ResponseWriter, token string) (*unstructured.Unstructured, bool) {
	return shared.FindResourceOrRespond(
		w,
		func() (*unstructured.Unstructured, error) {
			return FindSessionByToken(token)
		},
		constants.ErrSessionNotFoundForToken,
		token,
	)
}
