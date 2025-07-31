package bridges

import (
	"fmt"
	"net/http"

	bridgeResource "github.com/plsyro/data-pkg/resources/bridge"
	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/clients/shared"
	"github.com/plsyro/rest-pkg/constants"
	bridgeEndpoints "github.com/plsyro/rest-pkg/endpoints/resources/bridges"
	response "github.com/plsyro/rest-pkg/response"
	requestUtils "github.com/plsyro/rest-pkg/utils/request"
)

// Client provides a unified interface for bridge operations
type Client struct {
	sharedClient *shared.Client
}

// NewClient creates a new Client instance
func NewClient() *Client {
	return &Client{
		sharedClient: shared.NewClient(),
	}
}

// CreateBridge creates a new bridge
func (c *Client) CreateBridge(bridge *bridgeResource.BridgeAsResource) *response.GenericResponse {
	return c.sharedClient.CreateResource(
		base.EXPORTER,
		base.V1,
		bridgeEndpoints.CREATE_BRIDGE,
		bridge,
		constants.RESOURCE_TYPE_BRIDGE,
	)
}

// GetBridgeByName retrieves a bridge by name
func (c *Client) GetBridgeByName(name string) (*bridgeResource.BridgeAsResource, error) {
	apiEndpoint := c.sharedClient.FormatEndpoint(string(bridgeEndpoints.GET_BRIDGE), name)
	request := requestUtils.CreateGenericRequest(base.GET, base.EXPORTER, base.V1, base.Endpoint(apiEndpoint))
	requestURL, err := request.GenerateURL()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GENERATE_REQUEST_URL_FOR), constants.RESOURCE_TYPE_BRIDGE, name, err)
	}

	req, err := http.NewRequest(constants.HTTP_METHOD_GET, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for bridge %s: %w", name, err)
	}

	bridge, err := shared.DoRequest[bridgeResource.BridgeAsResource](req)
	if err != nil {
		return nil, fmt.Errorf("failed to get bridge %s: %w", name, err)
	}
	return bridge, nil
}

// GetAllBridges retrieves all bridges
func (c *Client) GetAllBridges() ([]*bridgeResource.BridgeAsResource, error) {
	request := requestUtils.CreateGenericRequest(base.GET, base.EXPORTER, base.V1, bridgeEndpoints.GET_ALL_BRIDGES)
	requestURL, err := request.GenerateURL()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GENERATE_REQUEST_URL), err)
	}

	req, err := http.NewRequest(constants.HTTP_METHOD_GET, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	bridges, err := shared.DoRequestList[*bridgeResource.BridgeAsResource](req, constants.RESPONSE_ITEMS_KEY)
	if err != nil {
		return nil, fmt.Errorf("failed to get bridges: %w", err)
	}
	return bridges, nil
}

// PatchBridge updates a bridge
func (c *Client) PatchBridge(name string, body map[string]interface{}) *response.GenericResponse {
	return c.sharedClient.PatchResource(
		bridgeEndpoints.PATCH_BRIDGE,
		name,
		body,
	)
}

// DeleteBridge deletes a bridge
func (c *Client) DeleteBridge(name string) *response.GenericResponse {
	return c.sharedClient.DeleteResource(
		bridgeEndpoints.DELETE_BRIDGE,
		name,
	)
}
