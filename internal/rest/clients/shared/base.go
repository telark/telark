package shared

import (
	"fmt"
	"net/http"

	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/constants"
	response "github.com/plsyro/rest-pkg/response"
	requestUtils "github.com/plsyro/rest-pkg/utils/request"
	responseUtils "github.com/plsyro/rest-pkg/utils/response"
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

func GetResourceByNameTyped[T any](client *BaseResourceClient, endpoint base.Endpoint, name string) (*T, error) {
	apiEndpoint := client.FormatEndpoint(string(endpoint), name)
	request := requestUtils.CreateGenericRequest(base.GET, client.service, base.V1, base.Endpoint(apiEndpoint))
	requestURL, err := request.GenerateURL()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GENERATE_REQUEST_URL_FOR), client.resourceType, name, err)
	}

	req, err := http.NewRequest(constants.HTTP_METHOD_GET, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_CREATE_REQUEST), client.resourceType, name, err)
	}

	resource, err := DoRequest[T](req)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GET_RESOURCE), client.resourceType, name, err)
	}
	return resource, nil
}

func GetAllResourcesTyped[T any](client *BaseResourceClient, endpoint base.Endpoint) ([]T, error) {
	request := requestUtils.CreateGenericRequest(base.GET, client.service, base.V1, endpoint)
	requestURL, err := request.GenerateURL()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GENERATE_REQUEST_URL), err)
	}

	req, err := http.NewRequest(constants.HTTP_METHOD_GET, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_CREATE_REQUEST_GENERIC), err)
	}

	resources, err := DoRequestList[T](req, constants.RESPONSE_ITEMS_KEY)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GET_RESOURCES), client.resourceType, err)
	}
	return resources, nil
}

func (c *BaseResourceClient) DeleteResourceNoParams(endpoint base.Endpoint) *response.GenericResponse {
	request := requestUtils.CreateGenericRequest(base.DELETE, c.service, base.V1, endpoint)
	url, err := request.GenerateURL()
	if err != nil {
		return responseUtils.LogAndReturnResponse(constants.HTTP_STATUS_INTERNAL_SERVER_ERROR, response.OPERATION_ERROR, string(constants.ERROR_FAILED_GENERATE_URL), nil, err)
	}

	httpRequest, err := http.NewRequest(constants.HTTP_METHOD_DELETE, url, nil)
	if err != nil {
		return responseUtils.LogAndReturnResponse(constants.HTTP_STATUS_INTERNAL_SERVER_ERROR, response.OPERATION_ERROR, string(constants.ERROR_FAILED_CREATE_DELETE_REQUEST), nil, err)
	}

	httpClient := &http.Client{}
	resp, err := httpClient.Do(httpRequest)
	if err != nil {
		return responseUtils.LogAndReturnResponse(constants.HTTP_STATUS_INTERNAL_SERVER_ERROR, response.OPERATION_ERROR, string(constants.ERROR_FAILED_EXECUTE_DELETE_REQUEST), nil, err)
	}

	return responseUtils.ReadAndParseGenericResponse(resp)
}

func (c *BaseResourceClient) PostResourceNoPayload(endpoint base.Endpoint) (*response.GenericResponse, error) {
	request := requestUtils.CreateGenericRequest(base.POST, c.service, base.V1, endpoint)
	requestURL, err := request.GenerateURL()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GENERATE_REQUEST_URL), err)
	}

	req, err := http.NewRequest(constants.HTTP_METHOD_POST, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_CREATE_REQUEST_GENERIC), err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_EXECUTE_REQUEST), err)
	}

	return responseUtils.ReadAndParseGenericResponse(resp), nil
}

func (c *BaseResourceClient) FormatEndpoint(template, name string) string {
	return c.sharedClient.FormatEndpoint(template, name)
}
