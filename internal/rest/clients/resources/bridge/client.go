package bridge

import (
	bridgeresource "github.com/plsyro/data/resources/bridge"
	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/clients/shared"
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
	return shared.GetTyped[bridgeresource.BridgeAsResource](c.Client, eps.GetBridge,
		name)
}

func (c *Client) GetAllBridges() ([]*bridgeresource.BridgeAsResource, error) {
	return shared.GetListTyped[*bridgeresource.BridgeAsResource](c.Client,
		eps.GetAllBridges)
}

func (c *Client) PatchBridge(name string, body map[string]any) *response.GenericResponse {
	return c.Update(eps.PatchBridge, name, body)
}

func (c *Client) DeleteBridge(name string) *response.GenericResponse {
	return c.Delete(eps.DeleteBridge, name)
}
