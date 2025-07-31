package shared

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/plsyro/data-pkg/common"
	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/constants"
	restMapper "github.com/plsyro/rest-pkg/mappers/common"
	response "github.com/plsyro/rest-pkg/response"
	responseUtils "github.com/plsyro/rest-pkg/utils/response"
)

type Client struct {
	httpClient *http.Client
	service    base.Service
}

func New(service base.Service) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		service:    service,
	}
}

func (c *Client) Create(endpoint base.Endpoint, resource any) *response.GenericResponse {
	payload, err := marshalToJSON(restMapper.MapToJsonPayload(resource))
	if err != nil {
		return createErrorResponse(string(errors.ERROR_REST_MARSHALL_PAYLOAD), err)
	}

	resp, err := executeHTTPRequest(c, base.POST, endpoint, payload)
	if err != nil {
		return createErrorResponse(string(errors.ERROR_CREATE_RESOURCE), err)
	}

	return responseUtils.ReadAndParseGenericResponse(resp)
}

func (c *Client) Update(endpoint base.Endpoint, name string, body map[string]any) *response.GenericResponse {
	payload, err := marshalToJSON(body)
	if err != nil {
		return createErrorResponse(string(errors.ERROR_REST_MARSHALL_PAYLOAD), err)
	}

	resp, err := executeHTTPRequest(c, base.PATCH, substituteEndpointName(endpoint, name), payload)
	if err != nil {
		return createErrorResponse(string(errors.ERROR_UPDATE_RESOURCE), err)
	}

	return responseUtils.ReadAndParseGenericResponse(resp)
}

func (c *Client) Delete(endpoint base.Endpoint, name string) *response.GenericResponse {
	resp, err := executeHTTPRequest(c, base.DELETE, substituteEndpointName(endpoint, name), nil)
	if err != nil {
		message := fmt.Sprintf(string(errors.ERROR_DELETE_RESOURCE), name, err)
		return createErrorResponse(message, err)
	}

	return responseUtils.ReadAndParseGenericResponse(resp)
}

func (c *Client) Get(endpoint base.Endpoint, name string) (*response.GenericResponse, error) {
	resp, err := executeHTTPRequest(c, base.GET, substituteEndpointName(endpoint, name), nil)
	if err != nil {
		return nil, err
	}

	apiResponse := responseUtils.ReadAndParseGenericResponse(resp)
	if apiResponse.Status != common.STATUS_OK {
		return nil, fmt.Errorf(string(constants.ERROR_UNEXPECTED_STATUS), apiResponse.Status, apiResponse.Message)
	}

	return apiResponse, nil
}

func (c *Client) GetList(endpoint base.Endpoint) ([]any, error) {
	resp, err := executeHTTPRequest(c, base.GET, endpoint, nil)
	if err != nil {
		return nil, err
	}

	return parseListResponse[any](resp)
}

func GetTyped[T any](client *Client, endpoint base.Endpoint, name string) (*T, error) {
	resp, err := executeHTTPRequest(client, base.GET, substituteEndpointName(endpoint, name), nil)
	if err != nil {
		return nil, err
	}

	return parseSingleResponse[T](resp)
}

func GetListTyped[T any](client *Client, endpoint base.Endpoint) ([]T, error) {
	resp, err := executeHTTPRequest(client, base.GET, endpoint, nil)
	if err != nil {
		return nil, err
	}

	return parseListResponse[T](resp)
}

func (c *Client) Post(endpoint base.Endpoint) (*response.GenericResponse, error) {
	resp, err := executeHTTPRequest(c, base.POST, endpoint, nil)
	if err != nil {
		return nil, err
	}

	return responseUtils.ReadAndParseGenericResponse(resp), nil
}

func (c *Client) DeleteNoParams(endpoint base.Endpoint) *response.GenericResponse {
	resp, err := executeHTTPRequest(c, base.DELETE, endpoint, nil)
	if err != nil {
		message := fmt.Sprintf(string(errors.ERROR_DELETE_RESOURCE), "", err)
		return createErrorResponse(message, err)
	}

	return responseUtils.ReadAndParseGenericResponse(resp)
}

func (c *Client) FormatEndpoint(template, name string) string {
	return strings.Replace(template, constants.ENDPOINT_NAME_PLACEHOLDER, name, 1)
}
