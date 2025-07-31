package shared

import (
	"fmt"
	"net/http"

	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/constants"
	response "github.com/plsyro/rest-pkg/response"
	requestUtils "github.com/plsyro/rest-pkg/utils/request"
)

type BaseResourceClient struct {
	sharedClient *Client
	service      base.Service
	resourceType string
}

func NewBaseResourceClient(service base.Service, resourceType string) *BaseResourceClient {
	return &BaseResourceClient{
		sharedClient: NewClient(),
		service:      service,
		resourceType: resourceType,
	}
}

func (c *BaseResourceClient) CreateResource(endpoint base.Endpoint, resource any) *response.GenericResponse {
	return c.sharedClient.CreateResource(
		c.service,
		base.V1,
		endpoint,
		resource,
		c.resourceType,
	)
}

func (c *BaseResourceClient) PatchResource(endpoint base.Endpoint, name string, body map[string]interface{}) *response.GenericResponse {
	return c.sharedClient.PatchResource(endpoint, name, body)
}

func (c *BaseResourceClient) DeleteResource(endpoint base.Endpoint, name string) *response.GenericResponse {
	return c.sharedClient.DeleteResource(endpoint, name)
}

func (c *BaseResourceClient) GetResourceByName(endpoint base.Endpoint, name string) (*response.GenericResponse, error) {
	return c.sharedClient.GetResourceByName(endpoint, name, c.resourceType)
}

func (c *BaseResourceClient) GetAllResources(endpoint base.Endpoint) ([]any, error) {
	request := requestUtils.CreateGenericRequest(base.GET, c.service, base.V1, endpoint)
	requestURL, err := request.GenerateURL()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GENERATE_REQUEST_URL), err)
	}

	req, err := http.NewRequest(constants.HTTP_METHOD_GET, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_CREATE_REQUEST_GENERIC), err)
	}

	items, err := DoRequestList[any](req, constants.RESPONSE_ITEMS_KEY)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GET_RESOURCES), c.resourceType, err)
	}
	return items, nil
}

func (c *BaseResourceClient) FormatEndpoint(template, name string) string {
	return c.sharedClient.FormatEndpoint(template, name)
}
