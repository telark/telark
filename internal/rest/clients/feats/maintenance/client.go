package maintenance

import (
	"github.com/plsyro/data-pkg/feats/maintenance"
	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/clients/shared"
	maintenanceEndpoints "github.com/plsyro/rest-pkg/endpoints/feats/maintenance/base"
	grouperEndpoints "github.com/plsyro/rest-pkg/endpoints/feats/maintenance/grouper"
	response "github.com/plsyro/rest-pkg/response"
)

type Client struct {
	*shared.Client
}

func NewClient(useExporter bool) *Client {
	var service base.Service
	if useExporter {
		service = base.EXPORTER
	} else {
		service = base.CONFIGURATOR
	}

	return &Client{
		Client: shared.New(service),
	}
}

func (c *Client) CreateMaintenanceFeat(maintenance *maintenance.MaintenanceAsFeature) *response.GenericResponse {
	return c.Create(maintenanceEndpoints.CREATE_MAINTENANCE_FEAT, maintenance)
}

func (c *Client) PatchMaintenanceFeat(name string, body map[string]any) *response.GenericResponse {
	return c.Update(maintenanceEndpoints.PATCH_MAINTENANCE_FEAT, name, body)
}

func (c *Client) GetMaintenanceFeatByName(name string) (*maintenance.MaintenanceAsFeature, error) {
	return shared.GetTyped[maintenance.MaintenanceAsFeature](c.Client, maintenanceEndpoints.GET_MAINTENANCE_FEAT, name)
}

func (c *Client) DeleteMaintenanceFeat(name string) *response.GenericResponse {
	return c.Delete(maintenanceEndpoints.DELETE_MAINTENANCE_FEAT, name)
}

func (c *Client) EnableGrouperMaintenance(body map[string]interface{}) *response.GenericResponse {
	return c.Create(grouperEndpoints.ENABLE_GROUPER_MAINTENANCE_FEAT, body)
}

func (c *Client) UpdateGrouperMaintenance(body map[string]interface{}) *response.GenericResponse {
	return c.Create(grouperEndpoints.UPDATE_GROUPER_MAINTENANCE_FEAT, body)
}

func (c *Client) RemoveGrouperMaintenance() *response.GenericResponse {
	return c.DeleteNoParams(grouperEndpoints.REMOVE_GROUPER_MAINTENANCE_FEAT)
}
