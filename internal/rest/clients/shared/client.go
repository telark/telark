package shared

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/constants"
	restMapper "github.com/plsyro/rest-pkg/mappers/common"
	response "github.com/plsyro/rest-pkg/response"
	requestUtils "github.com/plsyro/rest-pkg/utils/request"
	responseUtils "github.com/plsyro/rest-pkg/utils/response"
)

// APIResponse represents the standard API response structure
type APIResponse struct {
	Status int `json:"status"`
	Data   any `json:"data"`
}

// Client provides a unified interface for making HTTP requests
type Client struct{}

// NewClient creates a new Client instance
func NewClient() *Client {
	return &Client{}
}

// DoRequest sends an HTTP request and unmarshals the response into the provided type T
func DoRequest[T any](req *http.Request) (*T, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_EXECUTE_REQUEST), err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_READ_RESPONSE_BODY_GENERIC), err)
	}

	if resp.StatusCode != constants.HTTP_STATUS_OK {
		return nil, fmt.Errorf(string(constants.ERROR_UNEXPECTED_STATUS_CODE), resp.StatusCode)
	}

	var apiResp APIResponse
	apiResp.Data = new(T)
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_UNMARSHAL_RESPONSE), err)
	}

	// Unmarshal Data into T
	dataBytes, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_MARSHAL_DATA), err)
	}
	var result T
	if err := json.Unmarshal(dataBytes, &result); err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_UNMARSHAL_DATA), err)
	}

	return &result, nil
}

// DoRequestList handles list responses (where items are under a key in Data)
func DoRequestList[T any](req *http.Request, itemsKey string) ([]T, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_EXECUTE_REQUEST), err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_READ_RESPONSE_BODY_GENERIC), err)
	}

	if resp.StatusCode != constants.HTTP_STATUS_OK {
		return nil, fmt.Errorf(string(constants.ERROR_UNEXPECTED_STATUS_CODE), resp.StatusCode)
	}

	var apiResp map[string]any
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_UNMARSHAL_RESPONSE), err)
	}

	data, ok := apiResp[constants.RESPONSE_DATA_KEY].(map[string]any)
	if !ok {
		return nil, fmt.Errorf(string(constants.ERROR_UNEXPECTED_DATA_FIELD_FORMAT))
	}
	itemsRaw, ok := data[itemsKey]
	if !ok {
		return nil, fmt.Errorf(string(constants.ERROR_ITEMS_FIELD_MISSING))
	}
	itemsJSON, err := json.Marshal(itemsRaw)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_MARSHAL_ITEMS), err)
	}
	var items []T
	if err := json.Unmarshal(itemsJSON, &items); err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_UNMARSHAL_ITEMS), err)
	}
	return items, nil
}

// CreateResource creates a new resource using POST request
func (c *Client) CreateResource(
	service base.Service,
	version base.Version,
	endpoint base.Endpoint,
	resource any,
	resourceName string,
) *response.GenericResponse {
	// Prepare Payload
	mappedResource := restMapper.MapToJsonPayload(resource)
	payload, err := json.Marshal(mappedResource)
	if err != nil {
		message := fmt.Sprintf(string(constants.ERROR_FAILED_MARSHAL_PAYLOAD), resourceName)
		return responseUtils.LogAndReturnResponse(constants.HTTP_STATUS_INTERNAL_SERVER_ERROR, response.OPERATION_ERROR, message, nil, err)
	}

	// Prepare Request
	request := requestUtils.CreateGenericRequestWithPayload(base.POST, service, version, endpoint, payload)

	// Send POST Request
	url, err := request.GenerateURL()
	if err != nil {
		return responseUtils.LogAndReturnResponse(constants.HTTP_STATUS_INTERNAL_SERVER_ERROR, response.OPERATION_ERROR, string(constants.ERROR_FAILED_GENERATE_URL), nil, err)
	}
	resp, err := http.Post(url, constants.CONTENT_TYPE_JSON, bytes.NewBuffer(request.Payload))
	if err != nil {
		message := fmt.Sprintf(string(constants.ERROR_FAILED_SEND_REQUEST), resourceName)
		return responseUtils.LogAndReturnResponse(constants.HTTP_STATUS_INTERNAL_SERVER_ERROR, response.OPERATION_ERROR, message, nil, err)
	}

	// Parse and return generic response
	return responseUtils.ReadAndParseGenericResponse(resp)
}

// PatchResource updates a resource using PATCH request
func (c *Client) PatchResource(
	endpoint base.Endpoint,
	name string,
	body map[string]any,
) *response.GenericResponse {
	// Prepare Payload
	payload, err := json.Marshal(body)
	if err != nil {
		return responseUtils.LogAndReturnResponse(constants.HTTP_STATUS_INTERNAL_SERVER_ERROR, response.OPERATION_ERROR, string(constants.ERROR_FAILED_MARSHAL_PATCH_PAYLOAD), nil, err)
	}

	// Prepare Endpoint
	apiEndpoint := strings.Replace(string(endpoint), constants.ENDPOINT_NAME_PLACEHOLDER, name, 1)

	// Prepare Request
	request := requestUtils.CreateGenericRequestWithPayload(base.PATCH, base.EXPORTER, base.V1, base.Endpoint(apiEndpoint), payload)
	url, err := request.GenerateURL()
	if err != nil {
		return responseUtils.LogAndReturnResponse(constants.HTTP_STATUS_INTERNAL_SERVER_ERROR, response.OPERATION_ERROR, string(constants.ERROR_FAILED_GENERATE_URL), nil, err)
	}

	// Create PATCH Request
	httpRequest, err := http.NewRequest(constants.HTTP_METHOD_PATCH, url, bytes.NewBuffer(request.Payload))
	if err != nil {
		return responseUtils.LogAndReturnResponse(constants.HTTP_STATUS_INTERNAL_SERVER_ERROR, response.OPERATION_ERROR, string(constants.ERROR_FAILED_CREATE_PATCH_REQUEST), nil, err)
	}
	httpRequest.Header.Set("Content-Type", constants.CONTENT_TYPE_JSON)

	client := &http.Client{}
	resp, err := client.Do(httpRequest)
	if err != nil {
		return responseUtils.LogAndReturnResponse(constants.HTTP_STATUS_INTERNAL_SERVER_ERROR, response.OPERATION_ERROR, string(constants.ERROR_FAILED_EXECUTE_PATCH_REQUEST), nil, err)
	}

	return responseUtils.ReadAndParseGenericResponse(resp)
}

// DeleteResource deletes a resource using DELETE request
func (c *Client) DeleteResource(endpoint base.Endpoint, name string) *response.GenericResponse {
	// Prepare Endpoint
	apiEndpoint := strings.Replace(string(endpoint), constants.ENDPOINT_NAME_PLACEHOLDER, name, 1)

	// Prepare Request
	request := requestUtils.CreateGenericRequest(base.DELETE, base.EXPORTER, base.V1, base.Endpoint(apiEndpoint))

	// Send Delete Request
	url, err := request.GenerateURL()
	if err != nil {
		return responseUtils.LogAndReturnResponse(constants.HTTP_STATUS_INTERNAL_SERVER_ERROR, response.OPERATION_ERROR, string(constants.ERROR_FAILED_GENERATE_URL), nil, err)
	}

	// Create DELETE request manually
	httpRequest, err := http.NewRequest(constants.HTTP_METHOD_DELETE, url, nil)
	if err != nil {
		return responseUtils.LogAndReturnResponse(constants.HTTP_STATUS_INTERNAL_SERVER_ERROR, response.OPERATION_ERROR, string(constants.ERROR_FAILED_CREATE_DELETE_REQUEST), nil, err)
	}

	httpClient := &http.Client{}
	resp, err := httpClient.Do(httpRequest)
	if err != nil {
		return responseUtils.LogAndReturnResponse(constants.HTTP_STATUS_INTERNAL_SERVER_ERROR, response.OPERATION_ERROR, string(constants.ERROR_FAILED_EXECUTE_DELETE_REQUEST), nil, err)
	}

	// Parse and return generic response
	return responseUtils.ReadAndParseGenericResponse(resp)
}

// GetResourceByName retrieves a resource by name using GET request
func (c *Client) GetResourceByName(
	endpoint base.Endpoint,
	name string,
	resourceType string,
) (*response.GenericResponse, error) {
	// Prepare Request
	apiEndpoint := strings.Replace(string(endpoint), constants.ENDPOINT_NAME_PLACEHOLDER, name, 1)
	request := requestUtils.CreateGenericRequest(base.GET, base.EXPORTER, base.V1, base.Endpoint(apiEndpoint))

	// Generate Request URL
	requestURL, err := request.GenerateURL()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GENERATE_REQUEST_URL_FOR), resourceType, name, err)
	}

	// Send GET Request
	response, err := http.Get(requestURL)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_SEND_GET_REQUEST), resourceType, name, err)
	}

	apiResponse := responseUtils.ReadAndParseGenericResponse(response)

	// Validate API Response Status
	if apiResponse.Status != constants.HTTP_STATUS_OK {
		return nil, fmt.Errorf(string(constants.ERROR_NON_SUCCESS_STATUS_FOR), apiResponse.Status, resourceType, name)
	}

	return apiResponse, nil
}

// GetResourceByNameAndUnmarshal retrieves a resource by name and unmarshals it into the target type
func GetResourceByNameAndUnmarshal[T any](
	client *Client,
	endpoint base.Endpoint,
	name string,
	resourceType string,
	target *T,
) error {
	apiResponse, err := client.GetResourceByName(endpoint, name, resourceType)
	if err != nil {
		return err
	}
	dataBytes, err := json.Marshal(apiResponse.Data)
	if err != nil {
		return fmt.Errorf(string(constants.ERROR_FAILED_SERIALIZE_DATA), resourceType, name, err)
	}
	if err := json.Unmarshal(dataBytes, target); err != nil {
		return fmt.Errorf(string(constants.ERROR_FAILED_MAP_DATA_FIELD), resourceType, name, err)
	}
	return nil
}

// FormatEndpoint formats an endpoint template by replacing placeholders
func (c *Client) FormatEndpoint(template, name string) string {
	return strings.Replace(template, constants.ENDPOINT_NAME_PLACEHOLDER, name, 1)
}
