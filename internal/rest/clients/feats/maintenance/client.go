package maintenance

import (
	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/clients/shared"
	maintenanceEndpoints "github.com/plsyro/rest-pkg/endpoints/feats/maintenance/grouper"
	response "github.com/plsyro/rest-pkg/response"
)

type Client struct {
	*shared.Client
}

func NewClient() *Client {
	return &Client{
		Client: shared.New(base.CONFIGURATOR),
	}
}

func (c *Client) EnableGrouperMaintenance(body map[string]interface{}) *response.GenericResponse {
	return c.Create(maintenanceEndpoints.ENABLE_GROUPER_MAINTENANCE_FEAT, body)
}

func (c *Client) UpdateGrouperMaintenance(body map[string]interface{}) *response.GenericResponse {
	return c.Create(maintenanceEndpoints.UPDATE_GROUPER_MAINTENANCE_FEAT, body)
}

func (c *Client) RemoveGrouperMaintenance() *response.GenericResponse {
	return c.DeleteNoParams(maintenanceEndpoints.REMOVE_GROUPER_MAINTENANCE_FEAT)
}
