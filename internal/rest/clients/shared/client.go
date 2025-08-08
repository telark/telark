package shared

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/plsyro/data/errors"
	globalshared "github.com/plsyro/data/shared"
	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/constants"
	restmapper "github.com/plsyro/rest/mappers"
	"github.com/plsyro/rest/response"
	responseutils "github.com/plsyro/rest/utils/response"
)

func New(service base.Service) *Client {
	return NewWithConfig(service, DefaultClientConfig())
}

func NewWithConfig(service base.Service, config *ClientConfig) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: config.Timeout},
		service:    service,
		config:     config,
	}
}

func (c *Client) executeRequest(
	method base.Method,
	endpoint base.Endpoint,
	payload any,
) *response.GenericResponse {
	var jsonPayload []byte
	var err error
	if payload != nil {
		jsonPayload, err = marshalToJSON(payload)
		if err != nil {
			return createErrorResponse(string(errors.ErrRestMarshalPayload), err)
		}
	}

	//nolint:bodyclose // responseutils.ReadAndParseGenericResponse handles closing
	resp, err := executeHTTPRequest(c, method, endpoint, jsonPayload)
	if err != nil {
		return createErrorResponse(string(errors.ErrCreateResource), err)
	}

	return responseutils.ReadAndParseGenericResponse(resp)
}

func (c *Client) executeRequestWithError(
	method base.Method,
	endpoint base.Endpoint,
	payload any,
) (*response.GenericResponse, error) {
	var jsonPayload []byte
	var err error
	if payload != nil {
		jsonPayload, err = marshalToJSON(payload)
		if err != nil {
			return nil, fmt.Errorf(string(errors.ErrRestMarshalPayload), err)
		}
	}

	//nolint:bodyclose // responseutils.ReadAndParseGenericResponse handles closing
	resp, err := executeHTTPRequest(c, method, endpoint, jsonPayload)
	if err != nil {
		return nil, err
	}

	apiResponse := responseutils.ReadAndParseGenericResponse(resp)
	if apiResponse.Status != globalshared.StatusOK {
		return nil, fmt.Errorf(
			string(constants.ErrUnexpectedStatus),
			apiResponse.Status,
			apiResponse.Message,
		)
	}

	return apiResponse, nil
}

func (c *Client) Create(endpoint base.Endpoint, resource any) *response.GenericResponse {
	mappedPayload, err := restmapper.MapToJSONPayload(resource)
	if err != nil {
		return createErrorResponse(string(errors.ErrRestMarshalPayload), err)
	}

	return c.executeRequest(base.Post, endpoint, mappedPayload)
}

func (c *Client) Update(
	endpoint base.Endpoint,
	name string,
	body map[string]any,
) *response.GenericResponse {
	substitutedEndpoint := substituteEndpointName(endpoint, name)
	return c.executeRequest(base.Patch, substitutedEndpoint, body)
}

func (c *Client) Delete(endpoint base.Endpoint, name string) *response.GenericResponse {
	substitutedEndpoint := substituteEndpointName(endpoint, name)
	return c.executeRequest(base.Delete, substitutedEndpoint, nil)
}

func (c *Client) Get(endpoint base.Endpoint, name string) (*response.GenericResponse, error) {
	substitutedEndpoint := substituteEndpointName(endpoint, name)
	return c.executeRequestWithError(base.Get, substitutedEndpoint, nil)
}

func (c *Client) GetList(endpoint base.Endpoint) ([]any, error) {
	//nolint:bodyclose // defer responseutils.CloseResponseBody handles closing
	resp, err := executeHTTPRequest(c, base.Get, endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer responseutils.CloseResponseBody(resp)

	return parseListResponse[any](resp)
}

func (c *Client) Post(endpoint base.Endpoint) (*response.GenericResponse, error) {
	return c.executeRequestWithError(base.Post, endpoint, nil)
}

func (c *Client) DeleteNoParams(endpoint base.Endpoint) *response.GenericResponse {
	return c.executeRequest(base.Delete, endpoint, nil)
}

func (c *Client) PostAndParseGenericResponses(endpoint base.Endpoint) ([]response.GenericResponse, error) {
	//nolint:bodyclose // parseGenericResponseSlice handles closing
	resp, err := executeHTTPRequest(c, base.Post, endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer responseutils.CloseResponseBody(resp)

	return parseGenericResponseSlice(resp)
}

func GetTyped[T any](client *Client, endpoint base.Endpoint, name string) (*T, error) {
	//nolint:bodyclose // defer responseutils.CloseResponseBody handles closing
	resp, err := executeHTTPRequest(client, base.Get, substituteEndpointName(endpoint, name), nil)
	if err != nil {
		return nil, err
	}
	defer responseutils.CloseResponseBody(resp)

	return parseSingleResponse[T](resp)
}

func GetListTyped[T any](client *Client, endpoint base.Endpoint) ([]T, error) {
	//nolint:bodyclose // defer responseutils.CloseResponseBody handles closing
	resp, err := executeHTTPRequest(client, base.Get, endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer responseutils.CloseResponseBody(resp)

	return parseListResponse[T](resp)
}

func (*Client) FormatEndpoint(template, name string) string {
	return strings.Replace(
		template,
		string(constants.EndpointNamePlaceholder),
		name,
		constants.ReplaceCount,
	)
}

func (c *Client) GetService() base.Service {
	return c.service
}

func (c *Client) GetHTTPClient() *http.Client {
	return c.httpClient
}
