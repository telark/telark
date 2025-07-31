package bridges

import (
	bridgeResource "github.com/plsyro/data-pkg/resources/bridge"
	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/clients/shared"
	"github.com/plsyro/rest-pkg/constants"
	bridgeEndpoints "github.com/plsyro/rest-pkg/endpoints/resources/bridges"
	response "github.com/plsyro/rest-pkg/response"
)

type Client struct {
	*shared.BaseResourceClient
}

func NewClient() *Client {
	return &Client{
		BaseResourceClient: shared.NewBaseResourceClient(base.EXPORTER, constants.RESOURCE_TYPE_BRIDGE),
	}
}

func (c *Client) CreateBridge(bridge *bridgeResource.BridgeAsResource) *response.GenericResponse {
	return c.CreateResource(bridgeEndpoints.CREATE_BRIDGE, bridge)
}

func (c *Client) GetBridgeByName(name string) (*bridgeResource.BridgeAsResource, error) {
	return shared.GetResourceByNameTyped[bridgeResource.BridgeAsResource](c.BaseResourceClient, bridgeEndpoints.GET_BRIDGE, name)
}

func (c *Client) GetAllBridges() ([]*bridgeResource.BridgeAsResource, error) {
	return shared.GetAllResourcesTyped[*bridgeResource.BridgeAsResource](c.BaseResourceClient, bridgeEndpoints.GET_ALL_BRIDGES)
}

func (c *Client) PatchBridge(name string, body map[string]interface{}) *response.GenericResponse {
	return c.PatchResource(bridgeEndpoints.PATCH_BRIDGE, name, body)
}

func (c *Client) DeleteBridge(name string) *response.GenericResponse {
	return c.DeleteResource(bridgeEndpoints.DELETE_BRIDGE, name)
}
