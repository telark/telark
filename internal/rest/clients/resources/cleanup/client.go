package cleanup

import (
	resourcesshared "github.com/telark/data/resources/shared"
	"github.com/telark/rest/base"
	"github.com/telark/rest/clients/shared"
	"github.com/telark/rest/constants"
	eps "github.com/telark/rest/endpoints/resources/cleanup"
	"github.com/telark/rest/response"
)

func AddFinalizer(c *shared.Client, resourceType, id, name string) *response.GenericResponse {
	return c.Update(buildIDEndpoint(string(eps.AddFinalizer), resourceType, id), finalizerPayload(name))
}

func RemoveFinalizer(c *shared.Client, resourceType, id, name string) *response.GenericResponse {
	return c.Update(buildIDEndpoint(string(eps.RemoveFinalizer), resourceType, id), finalizerPayload(name))
}

func GetCleanupViewByID(c *shared.Client, resourceType, id string) (*resourcesshared.CleanupView, error) {
	return shared.GetTyped[resourcesshared.CleanupView](
		c, buildIDEndpoint(string(eps.GetCleanupViewByID), resourceType, id),
	)
}

func ListCleanupViews(c *shared.Client, resourceType string) ([]*resourcesshared.CleanupView, error) {
	return shared.GetListTyped[*resourcesshared.CleanupView](
		c, buildTypeEndpoint(string(eps.ListCleanupViews), resourceType),
	)
}

func buildIDEndpoint(template, resourceType, id string) base.Endpoint {
	ep := shared.SubstituteEndpointWithParam(template, constants.TypeParam, resourceType)
	return shared.SubstituteEndpointWithParam(string(ep), constants.IDParam, id)
}

func buildTypeEndpoint(template, resourceType string) base.Endpoint {
	return shared.SubstituteEndpointWithParam(template, constants.TypeParam, resourceType)
}

func finalizerPayload(name string) map[string]any {
	return map[string]any{constants.FieldFinalizerName: name}
}
