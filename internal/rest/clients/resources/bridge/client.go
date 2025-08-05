package bridge

import (
	bridgeResource "github.com/plsyro/data/resources/bridge"
	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/clients/shared"
	bridgeEndpoints "github.com/plsyro/rest/endpoints/resources/bridges"
	response "github.com/plsyro/rest/response"
)

type Client struct {
	*shared.Client
}

func NewClient() *Client {
	return &Client{
		Client: shared.New(base.Exporter),
	}
}

func (c *Client) CreateBridge(bridge *bridgeResource.BridgeAsResource) *response.GenericResponse {
	return c.Create(bridgeEndpoints.CreateBridge, bridge)
}

func (c *Client) GetBridgeByName(name string) (*bridgeResource.BridgeAsResource, error) {
	return shared.GetTyped[bridgeResource.BridgeAsResource](c.Client, bridgeEndpoints.GetBridge, name)
}

func (c *Client) GetAllBridges() ([]*bridgeResource.BridgeAsResource, error) {
	return shared.GetListTyped[*bridgeResource.BridgeAsResource](c.Client, bridgeEndpoints.GetAllBridges)
}

func (c *Client) PatchBridge(name string, body map[string]interface{}) *response.GenericResponse {
	return c.Update(bridgeEndpoints.PatchBridge, name, body)
}

func (c *Client) DeleteBridge(name string) *response.GenericResponse {
	return c.Delete(bridgeEndpoints.DeleteBridge, name)
}
