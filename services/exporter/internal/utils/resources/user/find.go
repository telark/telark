package user

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	dataerrors "github.com/telark/data/errors"
	metadata "github.com/telark/data/metadata/resources"
	"github.com/telark/exporter/internal/constants"
	resourcesshared "github.com/telark/exporter/internal/utils/resources/shared"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var ErrUserNotFound = errors.New(string(constants.ErrUserNotFound))

func listUsers() (*unstructured.UnstructuredList, error) {
	result := api.ListCustomResources(metadata.UserAsResourceMetadata)
	if result.Error != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedToListUsers), result.Error)
	}

	list, ok := result.Data.(*unstructured.UnstructuredList)
	if !ok {
		return nil, errors.New(string(dataerrors.ErrGetRes))
	}

	return list, nil
}

func findUserBySpecField(field string, matches func(string) bool) (*unstructured.Unstructured, error) {
	list, err := listUsers()
	if err != nil {
		return nil, err
	}

	for i := range list.Items {
		spec, exists := list.Items[i].Object[constants.SpecField].(map[string]any)
		if !exists {
			continue
		}
		if current, ok := spec[field].(string); ok && matches(current) {
			return &list.Items[i], nil
		}
	}

	return nil, ErrUserNotFound
}

func FindUserByUsername(username string) (*unstructured.Unstructured, error) {
	return findUserBySpecField(constants.FieldUsername, func(current string) bool { return current == username })
}

// Mailboxes are case-insensitive, and a login by email must land on one account.
func FindUserByEmail(email string) (*unstructured.Unstructured, error) {
	want := NormalizeEmail(email)
	return findUserBySpecField(constants.FieldEmail, func(current string) bool { return NormalizeEmail(current) == want })
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func FindUserByIdentity(provider, issuer, subject string) (*unstructured.Unstructured, error) {
	list, err := listUsers()
	if err != nil {
		return nil, err
	}

	for i := range list.Items {
		spec, exists := list.Items[i].Object[constants.SpecField].(map[string]any)
		if !exists {
			continue
		}
		identities, ok := spec[constants.FieldIdentities].([]any)
		if !ok {
			continue
		}
		for _, entry := range identities {
			id, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			if id[constants.FieldProvider] == provider &&
				id[constants.FieldIssuer] == issuer &&
				id[constants.FieldSubject] == subject {
				return &list.Items[i], nil
			}
		}
	}

	return nil, ErrUserNotFound
}

func respondUserLookup(
	w http.ResponseWriter,
	resource *unstructured.Unstructured,
	err error,
) (*unstructured.Unstructured, bool) {
	if err == nil {
		return resource, true
	}
	if errors.Is(err, ErrUserNotFound) {
		responseutils.LogAndSendResponse(w, http.StatusNotFound, response.OperationNotFound,
			string(constants.ErrUserNotFound), nil, nil)
		return nil, false
	}
	responseutils.LogAndSendResponse(w, http.StatusInternalServerError, response.OperationError,
		err.Error(), nil, err)
	return nil, false
}

func FindUserByEmailOrRespond(w http.ResponseWriter, email string) (*unstructured.Unstructured, bool) {
	resource, err := FindUserByEmail(email)
	return respondUserLookup(w, resource, err)
}

func FindUserByUsernameOrRespond(w http.ResponseWriter, username string) (*unstructured.Unstructured, bool) {
	resource, err := FindUserByUsername(username)
	return respondUserLookup(w, resource, err)
}

func FindUserByIdentityOrRespond(w http.ResponseWriter, provider, issuer, subject string) (*unstructured.Unstructured, bool) {
	resource, err := FindUserByIdentity(provider, issuer, subject)
	return respondUserLookup(w, resource, err)
}

func FindUserByIDOrRespond(w http.ResponseWriter, userID string) (*unstructured.Unstructured, bool) {
	return resourcesshared.FindResourceByIDOrRespond(w, userID, metadata.UserAsResourceMetadata, constants.ErrUserNotFound)
}
