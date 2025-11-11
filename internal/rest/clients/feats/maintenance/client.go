package maintenance

import (
	maintenanceresource "github.com/plsyro/data/feats/maintenance"
	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/clients/shared"
	"github.com/plsyro/rest/constants"
	maineps "github.com/plsyro/rest/endpoints/feats/maintenance/base"
	grpeps "github.com/plsyro/rest/endpoints/feats/maintenance/grouper"
	"github.com/plsyro/rest/response"
)

type Client struct {
	*shared.Client
}

func NewClient(useExporter bool) *Client { //nolint:revive //TODO: each eps will be handled by a different client in the future
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

func (c *Client) CreateMaintenanceFeat(
	maintenance *maintenanceresource.MaintenanceAsFeature,
) *response.GenericResponse {
	return c.Create(maineps.CreateMaintenanceFeat, maintenance)
}

func (c *Client) PatchMaintenanceFeat(
	name string,
	body map[string]any,
) *response.GenericResponse {
	ep := base.Endpoint(shared.SubstituteEndpointWithParam(
		string(maineps.PatchMaintenanceFeat),
		constants.NameParam,
		name,
	))
	return c.Update(ep, body)
}

func (c *Client) GetMaintenanceFeatByName(
	name string,
) (*maintenanceresource.MaintenanceAsFeature, error) {
	ep := base.Endpoint(shared.SubstituteEndpointWithParam(
		string(maineps.GetMaintenanceFeat),
		constants.NameParam,
		name,
	))
	return shared.GetTyped[maintenanceresource.MaintenanceAsFeature](c.Client, ep)
}

func (c *Client) DeleteMaintenanceFeat(name string) *response.GenericResponse {
	ep := base.Endpoint(shared.SubstituteEndpointWithParam(
		string(maineps.DeleteMaintenanceFeat),
		constants.NameParam,
		name,
	))
	return c.Delete(ep)
}

func (c *Client) EnableGrouperMaintenance(body map[string]any) *response.GenericResponse {
	return c.Create(grpeps.EnableGrouperMaintenanceFeat, body)
}

func (c *Client) UpdateGrouperMaintenance(body map[string]any) *response.GenericResponse {
	return c.Create(grpeps.UpdateGrouperMaintenanceFeat, body)
}

func (c *Client) RemoveGrouperMaintenance() *response.GenericResponse {
	return c.Delete(grpeps.RemoveGrouperMaintenanceFeat)
}
