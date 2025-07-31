package bridges

import (
	bridgeResource "github.com/plsyro/data-pkg/resources/bridge"
	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/clients/shared"
	bridgeEndpoints "github.com/plsyro/rest-pkg/endpoints/resources/bridges"
	response "github.com/plsyro/rest-pkg/response"
)

type Client struct {
	*shared.BaseClient
}

func NewClient() *Client {
	return &Client{
		BaseClient: shared.NewBaseClient(base.EXPORTER),
	}
}

func (c *Client) CreateBridge(bridge *bridgeResource.BridgeAsResource) *response.GenericResponse {
	return c.Create(bridgeEndpoints.CREATE_BRIDGE, bridge)
}

func (c *Client) GetBridgeByName(name string) (*bridgeResource.BridgeAsResource, error) {
	return shared.GetTypedBase[bridgeResource.BridgeAsResource](c.BaseClient, bridgeEndpoints.GET_BRIDGE, name)
}

func (c *Client) GetAllBridges() ([]*bridgeResource.BridgeAsResource, error) {
	return shared.GetListTypedBase[*bridgeResource.BridgeAsResource](c.BaseClient, bridgeEndpoints.GET_ALL_BRIDGES)
}

func (c *Client) PatchBridge(name string, body map[string]interface{}) *response.GenericResponse {
	return c.Update(bridgeEndpoints.PATCH_BRIDGE, name, body)
}

func (c *Client) DeleteBridge(name string) *response.GenericResponse {
	return c.Delete(bridgeEndpoints.DELETE_BRIDGE, name)
}
