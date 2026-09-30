package session

import (
	"errors"
	"net/http"

	authmetadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/kcore/crds/api"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/utils/auth/shared"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Endpoints that address a session by path parameter accept its name too, so a
// device can be revoked without its token ever leaving the server. Ownership is
// still checked by the caller.
func FindSessionByRef(ref string) (*unstructured.Unstructured, error) {
	return findSessionByName(ResolveSessionRef(ref))
}

func findSessionByName(name string) (*unstructured.Unstructured, error) {
	result := api.GetCustomResourceByName(name, authmetadata.SessionMetadata)
	if err := sharedutils.ErrorForResult(result, constants.ErrSessionNotFound); err != nil {
		return nil, err
	}

	resource, ok := result.Data.(*unstructured.Unstructured)
	if !ok {
		return nil, errors.New(string(constants.ErrSessionListFormatInvalid))
	}

	return resource, nil
}

func FindSessionsByUserID(userID string) ([]unstructured.Unstructured, error) {
	return shared.FindResourcesByUserID(
		authmetadata.SessionMetadata,
		constants.ErrSessionListFormatInvalid,
		userID,
	)
}

func FindSessionOrRespond(w http.ResponseWriter, ref string) (*unstructured.Unstructured, bool) {
	return shared.FindResourceOrRespond(
		w,
		func() (*unstructured.Unstructured, error) {
			return FindSessionByRef(ref)
		},
		constants.ErrSessionNotFound,
	)
}
