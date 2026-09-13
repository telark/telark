package session

import (
	"errors"
	"net/http"

	authmetadata "github.com/telark/data/metadata/auth"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/utils/auth/shared"
	"github.com/telark/kcore/crds/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// The resource is named after the token digest, so a token resolves in one
// get. An unknown token is indistinguishable from a missing one.
//
// Tokens only: this backs authentication, where accepting a resource name would
// make the name a credential.
func FindSessionByToken(token string) (*unstructured.Unstructured, error) {
	return findSessionByName(SessionName(token))
}

// Endpoints that address a session by path parameter accept its name too, so a
// device can be revoked without its token ever leaving the server. Ownership is
// still checked by the caller.
func FindSessionByRef(ref string) (*unstructured.Unstructured, error) {
	return findSessionByName(ResolveSessionRef(ref))
}

func findSessionByName(name string) (*unstructured.Unstructured, error) {
	result := api.GetCustomResourceByName(name, authmetadata.UserSessionMetadata)
	if result.Status != http.StatusOK {
		return nil, errors.New(string(constants.ErrSessionNotFound))
	}

	resource, ok := result.Data.(*unstructured.Unstructured)
	if !ok {
		return nil, errors.New(string(constants.ErrSessionListFormatInvalid))
	}

	return resource, nil
}

func FindSessionsByUserID(userID string) ([]unstructured.Unstructured, error) {
	return shared.FindResourcesByUserID(
		authmetadata.UserSessionMetadata,
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
