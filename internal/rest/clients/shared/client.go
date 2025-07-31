package shared

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/constants"
	restMapper "github.com/plsyro/rest-pkg/mappers/common"
	response "github.com/plsyro/rest-pkg/response"
	requestUtils "github.com/plsyro/rest-pkg/utils/request"
	responseUtils "github.com/plsyro/rest-pkg/utils/response"
)

// Client handles all HTTP operations with a simple, clear interface
type Client struct {
	httpClient *http.Client
	service    base.Service
}

// New creates a new client
func New(service base.Service) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		service:    service,
	}
}

// Helper functions to eliminate redundancy

// buildRequest creates a request and returns URL
func (c *Client) buildRequest(method base.Method, endpoint base.Endpoint, payload []byte) (string, error) {
	var req interface{}
	if payload != nil {
		req = requestUtils.CreateGenericRequestWithPayload(method, c.service, base.V1, endpoint, payload)
	} else {
		req = requestUtils.CreateGenericRequest(method, c.service, base.V1, endpoint)
	}

	url, err := req.(interface{ GenerateURL() (string, error) }).GenerateURL()
	if err != nil {
		return "", fmt.Errorf("failed to generate URL: %w", err)
	}

	return url, nil
}

// executeRequest executes an HTTP request and returns generic response
func (c *Client) executeRequest(method, url string, payload []byte) *response.GenericResponse {
	var resp *http.Response
	var err error

	if payload != nil {
		httpReq, err := http.NewRequest(method, url, bytes.NewBuffer(payload))
		if err != nil {
			return c.handleError("failed to create request", err)
		}
		httpReq.Header.Set("Content-Type", constants.CONTENT_TYPE_JSON)
		resp, err = c.httpClient.Do(httpReq)
	} else {
		resp, err = http.Post(url, constants.CONTENT_TYPE_JSON, nil)
	}

	if err != nil {
		return c.handleError("failed to execute request", err)
	}

	return responseUtils.ReadAndParseGenericResponse(resp)
}

// handleError creates a generic error response
func (c *Client) handleError(message string, err error) *response.GenericResponse {
	return responseUtils.LogAndReturnResponse(
		constants.HTTP_STATUS_INTERNAL_SERVER_ERROR,
		response.OPERATION_ERROR,
		message,
		nil,
		err,
	)
}

// marshalPayload marshals data to JSON
func (c *Client) marshalPayload(data any, errorMessage string) ([]byte, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errorMessage, err)
	}
	return payload, nil
}

// sendGetRequest sends a GET request and returns response
func (c *Client) sendGetRequest(url string) (*http.Response, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	return resp, nil
}

// readResponseBody reads and parses response body
func (c *Client) readResponseBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	return body, nil
}

// parseListResponse parses a list response
func (c *Client) parseListResponse(resp *http.Response) ([]any, error) {
	body, err := c.readResponseBody(resp)
	if err != nil {
		return nil, err
	}

	var apiResp map[string]any
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	data, ok := apiResp[constants.RESPONSE_DATA_KEY].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected response format")
	}

	itemsRaw, ok := data[constants.RESPONSE_ITEMS_KEY]
	if !ok {
		return nil, fmt.Errorf("items field missing")
	}

	itemsJSON, err := json.Marshal(itemsRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal items: %w", err)
	}

	var items []any
	if err := json.Unmarshal(itemsJSON, &items); err != nil {
		return nil, fmt.Errorf("failed to parse items: %w", err)
	}

	return items, nil
}

// parseTypedResponse parses a typed response
func parseTypedResponse[T any](client *Client, resp *http.Response) (*T, error) {
	body, err := client.readResponseBody(resp)
	if err != nil {
		return nil, err
	}

	var apiResp struct {
		Status int `json:"status"`
		Data   T   `json:"data"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &apiResp.Data, nil
}

// parseTypedListResponse parses a typed list response
func parseTypedListResponse[T any](client *Client, resp *http.Response) ([]T, error) {
	body, err := client.readResponseBody(resp)
	if err != nil {
		return nil, err
	}

	var apiResp map[string]any
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	data, ok := apiResp[constants.RESPONSE_DATA_KEY].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected response format")
	}

	itemsRaw, ok := data[constants.RESPONSE_ITEMS_KEY]
	if !ok {
		return nil, fmt.Errorf("items field missing")
	}

	itemsJSON, err := json.Marshal(itemsRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal items: %w", err)
	}

	var items []T
	if err := json.Unmarshal(itemsJSON, &items); err != nil {
		return nil, fmt.Errorf("failed to parse items: %w", err)
	}

	return items, nil
}

// Public methods

// Create sends a POST request to create a resource
func (c *Client) Create(endpoint base.Endpoint, resource any) *response.GenericResponse {
	payload, err := c.marshalPayload(restMapper.MapToJsonPayload(resource), "failed to marshal resource")
	if err != nil {
		return c.handleError("failed to marshal resource", err)
	}

	url, err := c.buildRequest(base.POST, endpoint, payload)
	if err != nil {
		return c.handleError("failed to generate URL", err)
	}

	return c.executeRequest(constants.HTTP_METHOD_POST, url, payload)
}

// Update sends a PATCH request to update a resource
func (c *Client) Update(endpoint base.Endpoint, name string, body map[string]any) *response.GenericResponse {
	payload, err := c.marshalPayload(body, "failed to marshal update body")
	if err != nil {
		return c.handleError("failed to marshal update body", err)
	}

	namedEndpoint := strings.Replace(string(endpoint), constants.ENDPOINT_NAME_PLACEHOLDER, name, 1)
	url, err := c.buildRequest(base.PATCH, base.Endpoint(namedEndpoint), payload)
	if err != nil {
		return c.handleError("failed to generate URL", err)
	}

	return c.executeRequest(constants.HTTP_METHOD_PATCH, url, payload)
}

// Delete sends a DELETE request to delete a resource
func (c *Client) Delete(endpoint base.Endpoint, name string) *response.GenericResponse {
	namedEndpoint := strings.Replace(string(endpoint), constants.ENDPOINT_NAME_PLACEHOLDER, name, 1)
	url, err := c.buildRequest(base.DELETE, base.Endpoint(namedEndpoint), nil)
	if err != nil {
		return c.handleError("failed to generate URL", err)
	}

	return c.executeRequest(constants.HTTP_METHOD_DELETE, url, nil)
}

// Get sends a GET request to retrieve a resource
func (c *Client) Get(endpoint base.Endpoint, name string) (*response.GenericResponse, error) {
	namedEndpoint := strings.Replace(string(endpoint), constants.ENDPOINT_NAME_PLACEHOLDER, name, 1)
	url, err := c.buildRequest(base.GET, base.Endpoint(namedEndpoint), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.sendGetRequest(url)
	if err != nil {
		return nil, err
	}

	apiResponse := responseUtils.ReadAndParseGenericResponse(resp)
	if apiResponse.Status != constants.HTTP_STATUS_OK {
		return nil, fmt.Errorf("unexpected status: %d", apiResponse.Status)
	}

	return apiResponse, nil
}

// GetList sends a GET request to retrieve all resources
func (c *Client) GetList(endpoint base.Endpoint) ([]any, error) {
	url, err := c.buildRequest(base.GET, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.sendGetRequest(url)
	if err != nil {
		return nil, err
	}

	return c.parseListResponse(resp)
}

// GetTyped sends a GET request and returns a typed response
func GetTyped[T any](client *Client, endpoint base.Endpoint, name string) (*T, error) {
	namedEndpoint := strings.Replace(string(endpoint), constants.ENDPOINT_NAME_PLACEHOLDER, name, 1)
	url, err := client.buildRequest(base.GET, base.Endpoint(namedEndpoint), nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.sendGetRequest(url)
	if err != nil {
		return nil, err
	}

	return parseTypedResponse[T](client, resp)
}

// GetListTyped sends a GET request and returns a typed list response
func GetListTyped[T any](client *Client, endpoint base.Endpoint) ([]T, error) {
	url, err := client.buildRequest(base.GET, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.sendGetRequest(url)
	if err != nil {
		return nil, err
	}

	return parseTypedListResponse[T](client, resp)
}

// Post sends a POST request without payload
func (c *Client) Post(endpoint base.Endpoint) (*response.GenericResponse, error) {
	url, err := c.buildRequest(base.POST, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(url, constants.CONTENT_TYPE_JSON, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	return responseUtils.ReadAndParseGenericResponse(resp), nil
}

// DeleteNoParams sends a DELETE request without parameters
func (c *Client) DeleteNoParams(endpoint base.Endpoint) *response.GenericResponse {
	url, err := c.buildRequest(base.DELETE, endpoint, nil)
	if err != nil {
		return c.handleError("failed to generate URL", err)
	}

	return c.executeRequest(constants.HTTP_METHOD_DELETE, url, nil)
}

// FormatEndpoint replaces placeholder with name
func (c *Client) FormatEndpoint(template, name string) string {
	return strings.Replace(template, constants.ENDPOINT_NAME_PLACEHOLDER, name, 1)
}
