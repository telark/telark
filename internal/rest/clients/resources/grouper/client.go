package grouper

import (
	"fmt"
	"net/http"

	"github.com/plsyro/data-pkg/resources/grouper"
	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/clients/shared"
	"github.com/plsyro/rest-pkg/constants"
	grouperEndpoints "github.com/plsyro/rest-pkg/endpoints/resources/groupers"
	response "github.com/plsyro/rest-pkg/response"
	requestUtils "github.com/plsyro/rest-pkg/utils/request"
)

// Client provides a unified interface for grouper operations
type Client struct {
	sharedClient *shared.Client
}

// NewClient creates a new Client instance
func NewClient() *Client {
	return &Client{
		sharedClient: shared.NewClient(),
	}
}

// CreateGrouper creates a new grouper
func (c *Client) CreateGrouper(grouperObj *grouper.GrouperAsResource) *response.GenericResponse {
	return c.sharedClient.CreateResource(
		base.EXPORTER,
		base.V1,
		grouperEndpoints.CREATE_GROUPER,
		grouperObj,
		constants.RESOURCE_TYPE_GROUPER,
	)
}

// GetGrouperByName retrieves a grouper by name
func (c *Client) GetGrouperByName(name string) (*grouper.GrouperAsResource, error) {
	apiEndpoint := c.sharedClient.FormatEndpoint(string(grouperEndpoints.GET_GROUPER), name)
	request := requestUtils.CreateGenericRequest(base.GET, base.EXPORTER, base.V1, base.Endpoint(apiEndpoint))
	requestURL, err := request.GenerateURL()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GENERATE_REQUEST_URL_FOR), constants.RESOURCE_TYPE_GROUPER, name, err)
	}

	req, err := http.NewRequest(constants.HTTP_METHOD_GET, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for grouper %s: %w", name, err)
	}

	grouperObj, err := shared.DoRequest[grouper.GrouperAsResource](req)
	if err != nil {
		return nil, fmt.Errorf("failed to get grouper %s: %w", name, err)
	}
	return grouperObj, nil
}

// GetAllGroupers retrieves all groupers
func (c *Client) GetAllGroupers() ([]*grouper.GrouperAsResource, error) {
	request := requestUtils.CreateGenericRequest(base.GET, base.EXPORTER, base.V1, grouperEndpoints.GET_ALL_GROUPERS)
	requestURL, err := request.GenerateURL()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GENERATE_REQUEST_URL), err)
	}

	req, err := http.NewRequest(constants.HTTP_METHOD_GET, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	groupers, err := shared.DoRequestList[*grouper.GrouperAsResource](req, constants.RESPONSE_ITEMS_KEY)
	if err != nil {
		return nil, fmt.Errorf("failed to get groupers: %w", err)
	}
	return groupers, nil
}

// PatchGrouper updates a grouper
func (c *Client) PatchGrouper(name string, body map[string]interface{}) *response.GenericResponse {
	return c.sharedClient.PatchResource(
		grouperEndpoints.PATCH_GROUPER,
		name,
		body,
	)
}

// DeleteGrouper deletes a grouper
func (c *Client) DeleteGrouper(name string) *response.GenericResponse {
	return c.sharedClient.DeleteResource(
		grouperEndpoints.DELETE_GROUPER,
		name,
	)
}
