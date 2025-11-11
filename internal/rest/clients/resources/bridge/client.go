package bridge

import (
	bridgeresource "github.com/plsyro/data/resources/bridge"
	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/clients/shared"
	"github.com/plsyro/rest/constants"
	eps "github.com/plsyro/rest/endpoints/resources/bridges"
	"github.com/plsyro/rest/response"
)

type Client struct {
	*shared.Client
}

func NewClient() *Client {
	return &Client{
		Client: shared.New(base.Exporter),
	}
}

func (c *Client) CreateBridge(bridge *bridgeresource.BridgeAsResource) *response.GenericResponse {
	return c.Create(eps.CreateBridge, bridge)
}

func (c *Client) GetBridgeByName(name string) (*bridgeresource.BridgeAsResource, error) {
	endpoint := base.Endpoint(shared.SubstituteEndpointWithParam(
		string(eps.GetBridge),
		constants.EndpointNamePlaceholder,
		name,
	))
	return shared.GetTyped[bridgeresource.BridgeAsResource](c.Client, endpoint)
}

func (c *Client) GetAllBridges() ([]*bridgeresource.BridgeAsResource, error) {
	return shared.GetListTyped[*bridgeresource.BridgeAsResource](c.Client,
		eps.GetAllBridges)
}

func (c *Client) PatchBridge(name string, body map[string]any) *response.GenericResponse {
	endpoint := base.Endpoint(shared.SubstituteEndpointWithParam(
		string(eps.PatchBridge),
		constants.EndpointNamePlaceholder,
		name,
	))
	return c.Update(endpoint, body)
}

func (c *Client) DeleteBridge(name string) *response.GenericResponse {
	ep := base.Endpoint(shared.SubstituteEndpointWithParam(
		string(eps.DeleteBridge),
		constants.EndpointNamePlaceholder,
		name,
	))
	return c.Delete(ep)
}
