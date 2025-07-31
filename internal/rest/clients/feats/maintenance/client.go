package maintenance

import (
	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/clients/shared"
	"github.com/plsyro/rest-pkg/constants"
	maintenanceEndpoints "github.com/plsyro/rest-pkg/endpoints/feats/maintenance/grouper"
	response "github.com/plsyro/rest-pkg/response"
)

type Client struct {
	*shared.BaseResourceClient
}

func NewClient() *Client {
	return &Client{
		BaseResourceClient: shared.NewBaseResourceClient(base.CONFIGURATOR, constants.RESOURCE_TYPE_MAINTENANCE),
	}
}

func (c *Client) EnableGrouperMaintenance(body map[string]interface{}) *response.GenericResponse {
	return c.CreateResource(maintenanceEndpoints.ENABLE_GROUPER_MAINTENANCE_FEAT, body)
}

func (c *Client) UpdateGrouperMaintenance(body map[string]interface{}) *response.GenericResponse {
	return c.CreateResource(maintenanceEndpoints.UPDATE_GROUPER_MAINTENANCE_FEAT, body)
}

func (c *Client) RemoveGrouperMaintenance() *response.GenericResponse {
	return c.DeleteResourceNoParams(maintenanceEndpoints.REMOVE_GROUPER_MAINTENANCE_FEAT)
}
