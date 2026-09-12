package cleanup

import (
	resourcesshared "github.com/telark/data/resources/shared"
	"github.com/telark/rest/clients/shared"
	"github.com/telark/rest/constants"
	eps "github.com/telark/rest/endpoints/resources/cleanup"
	"github.com/telark/rest/response"
)

func AddFinalizer(c *shared.Client, resourceType, id, name string) *response.GenericResponse {
	return byTypeAndID(c, resourceType, id).Update(eps.AddFinalizer, finalizerPayload(name))
}

func RemoveFinalizer(c *shared.Client, resourceType, id, name string) *response.GenericResponse {
	return byTypeAndID(c, resourceType, id).Update(eps.RemoveFinalizer, finalizerPayload(name))
}

func GetCleanupViewByID(c *shared.Client, resourceType, id string) (*resourcesshared.CleanupView, error) {
	return shared.GetTyped[resourcesshared.CleanupView](
		byTypeAndID(c, resourceType, id), eps.GetCleanupViewByID,
	)
}

func ListCleanupViews(c *shared.Client, resourceType string) ([]*resourcesshared.CleanupView, error) {
	return shared.GetListTyped[*resourcesshared.CleanupView](
		c.WithParams(map[string]string{constants.TypeParam: resourceType}), eps.ListCleanupViews,
	)
}

func byTypeAndID(c *shared.Client, resourceType, id string) *shared.Client {
	return c.WithParams(map[string]string{constants.TypeParam: resourceType, constants.IDParam: id})
}

func finalizerPayload(name string) map[string]any {
	return map[string]any{constants.FieldFinalizerName: name}
}
