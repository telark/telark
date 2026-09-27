package authz

import (
	"errors"

	dataerrors "github.com/telark/data/errors"
	"github.com/telark/data/metadata/base"
	metadata "github.com/telark/data/metadata/v1alpha1"
	groupdata "github.com/telark/data/resources/group"
	roledata "github.com/telark/data/resources/role"
	userdata "github.com/telark/data/resources/user"
	"github.com/telark/exporter/internal/constants"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/x-ware/authz"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// This service owns the custom resources, so it reads them directly instead of
// calling its own API back over the network.
type crdSource struct{}

var source authz.GrantSource = crdSource{}

// Tests have no apiserver; nothing else swaps the source.
func UseGrantSource(s authz.GrantSource) {
	source = s
}

func (crdSource) User(userID string) (*userdata.User, error) {
	return decode[userdata.User](getByName(userID, metadata.UserMetadata))
}

func (crdSource) Group(groupID string) (*groupdata.Group, error) {
	return decode[groupdata.Group](getByName(groupID, metadata.GroupMetadata))
}

func (crdSource) Role(roleID string) (*roledata.AccessRole, error) {
	return decode[roledata.AccessRole](getByName(roleID, metadata.AccessRoleMetadata))
}

// The finder utilities fold every failure into "not found"; authz must keep a
// missing record apart from an unreachable API server, so it reads the raw
// envelope.
func getByName(name string, md base.Metadata) (*unstructured.Unstructured, error) {
	result := api.GetCustomResourceByName(name, md)
	if apierrors.IsNotFound(result.Error) {
		return nil, authz.ErrNotFound
	}
	if result.Error != nil {
		return nil, result.Error
	}

	resource, ok := result.Data.(*unstructured.Unstructured)
	if !ok {
		return nil, errors.New(string(constants.ErrInvalidResourceTypeReturned))
	}

	return resource, nil
}

// Deletion is held open by the cleanup finalizer, so the record still reads;
// CollectGrants sees the projected timestamp and treats the record as gone.
func decode[T any](resource *unstructured.Unstructured, err error) (*T, error) {
	if err != nil {
		return nil, err
	}
	sharedutils.ProjectDeletionTimestamp(resource)
	return sharedutils.UnstructuredToStruct[T](
		resource,
		dataerrors.ErrGetRes,
		dataerrors.ErrGetRes,
		dataerrors.ErrRestUnmarshalResourceToJSON,
	)
}

func collectGrants(userID string) (authz.Grants, error) {
	return authz.CollectGrants(source, lg, userID)
}
