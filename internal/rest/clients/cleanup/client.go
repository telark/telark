package cleanup

import (
	resourcesshared "github.com/telark/telark/internal/data/resources/shared"
	"github.com/telark/telark/internal/rest/base"
	"github.com/telark/telark/internal/rest/clients/shared"
	"github.com/telark/telark/internal/rest/constants"
	eps "github.com/telark/telark/internal/rest/endpoints/cleanup"
	"github.com/telark/telark/internal/rest/response"
)

func AddFinalizer(c *shared.Client, resourceType, id, name string) *response.GenericResponse {
	return shared.ExecuteRequestWithHeaders(
		byTypeAndID(c, resourceType, id), base.Update, eps.AddFinalizer, finalizerPayload(name), nil,
	)
}

func RemoveFinalizer(c *shared.Client, resourceType, id, name string) *response.GenericResponse {
	return shared.ExecuteRequestWithHeaders(
		byTypeAndID(c, resourceType, id), base.Delete, eps.RemoveFinalizer, finalizerPayload(name), nil,
	)
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
