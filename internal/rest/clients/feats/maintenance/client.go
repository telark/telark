package maintenance

import (
	"github.com/plsyro/data/feats/maintenance"
	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/clients/shared"
	maintenanceEndpoints "github.com/plsyro/rest/endpoints/feats/maintenance/base"
	grouperEndpoints "github.com/plsyro/rest/endpoints/feats/maintenance/grouper"
	response "github.com/plsyro/rest/response"
)

type Client struct {
	*shared.Client
}

func NewClient(useExporter bool) *Client {
	var service base.Service
	if useExporter {
		service = base.Exporter
	} else {
		service = base.Configurator
	}

	return &Client{
		Client: shared.New(service),
	}
}

func (c *Client) CreateMaintenanceFeat(maintenance *maintenance.MaintenanceAsFeature) *response.GenericResponse {
	return c.Create(maintenanceEndpoints.CreateMaintenanceFeat, maintenance)
}

func (c *Client) PatchMaintenanceFeat(name string, body map[string]any) *response.GenericResponse {
	return c.Update(maintenanceEndpoints.PatchMaintenanceFeat, name, body)
}

func (c *Client) GetMaintenanceFeatByName(name string) (*maintenance.MaintenanceAsFeature, error) {
	return shared.GetTyped[maintenance.MaintenanceAsFeature](c.Client, maintenanceEndpoints.GetMaintenanceFeat, name)
}

func (c *Client) DeleteMaintenanceFeat(name string) *response.GenericResponse {
	return c.Delete(maintenanceEndpoints.DeleteMaintenanceFeat, name)
}

func (c *Client) EnableGrouperMaintenance(body map[string]interface{}) *response.GenericResponse {
	return c.Create(grouperEndpoints.EnableGrouperMaintenanceFeat, body)
}

func (c *Client) UpdateGrouperMaintenance(body map[string]interface{}) *response.GenericResponse {
	return c.Create(grouperEndpoints.UpdateGrouperMaintenanceFeat, body)
}

func (c *Client) RemoveGrouperMaintenance() *response.GenericResponse {
	return c.DeleteNoParams(grouperEndpoints.RemoveGrouperMaintenanceFeat)
}
