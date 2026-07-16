package authz

import (
	dataerrors "github.com/telark/data/errors"
	groupdata "github.com/telark/data/resources/group"
	roledata "github.com/telark/data/resources/role"
	userdata "github.com/telark/data/resources/user"
	grouputils "github.com/telark/exporter/internal/utils/resources/group"
	roleutils "github.com/telark/exporter/internal/utils/resources/role"
	userutils "github.com/telark/exporter/internal/utils/resources/user"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/x-ware/authz"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// This service owns the custom resources, so it reads them directly instead of
// calling its own API back over the network.
type crdSource struct{}

func (crdSource) User(userID string) (*userdata.UserAsResource, error) {
	return decode[userdata.UserAsResource](userutils.FindUserByID(userID))
}

func (crdSource) Group(groupID string) (*groupdata.GroupAsResource, error) {
	return decode[groupdata.GroupAsResource](grouputils.FindGroupByID(groupID))
}

func (crdSource) Role(roleID string) (*roledata.RoleAsResource, error) {
	return decode[roledata.RoleAsResource](roleutils.FindRoleByID(roleID))
}

func decode[T any](resource *unstructured.Unstructured, err error) (*T, error) {
	if err != nil {
		return nil, err
	}
	return sharedutils.UnstructuredToStruct[T](
		resource,
		dataerrors.ErrGetRes,
		dataerrors.ErrGetRes,
		dataerrors.ErrRestUnmarshalResourceToJSON,
	)
}

func collectGrants(userID string) (authz.Grants, error) {
	return authz.CollectGrants(crdSource{}, lg, userID)
}
