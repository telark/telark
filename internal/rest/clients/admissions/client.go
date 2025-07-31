package admissions

import (
	"fmt"
	"net/http"

	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/clients/shared"
	"github.com/plsyro/rest-pkg/constants"
	admissionsEndpoints "github.com/plsyro/rest-pkg/endpoints/admissions"
	response "github.com/plsyro/rest-pkg/response"
	requestUtils "github.com/plsyro/rest-pkg/utils/request"
	responseUtils "github.com/plsyro/rest-pkg/utils/response"
)

// Client provides a unified interface for admissions operations
type Client struct {
	sharedClient *shared.Client
}

// NewClient creates a new Client instance
func NewClient() *Client {
	return &Client{
		sharedClient: shared.NewClient(),
	}
}

// CreateAdmissionValidatingWebhook creates a new admission validating webhook
func (c *Client) CreateAdmissionValidatingWebhook(webhook interface{}) *response.GenericResponse {
	return c.sharedClient.CreateResource(
		base.ADMISSION_OPERATOR,
		base.V1,
		admissionsEndpoints.CREATE_ADMISSION_VALIDATING_WEBHOOK,
		webhook,
		constants.RESOURCE_TYPE_ADMISSION_WEBHOOK,
	)
}

// GetAdmissionValidatingWebhook retrieves an admission validating webhook by name
func (c *Client) GetAdmissionValidatingWebhook(name string) (*response.GenericResponse, error) {
	apiEndpoint := c.sharedClient.FormatEndpoint(string(admissionsEndpoints.GET_ADMISSION_VALIDATING_WEBHOOK), name)
	request := requestUtils.CreateGenericRequest(base.GET, base.ADMISSION_OPERATOR, base.V1, base.Endpoint(apiEndpoint))
	requestURL, err := request.GenerateURL()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GENERATE_REQUEST_URL_FOR), constants.RESOURCE_TYPE_ADMISSION_WEBHOOK, name, err)
	}

	req, err := http.NewRequest(constants.HTTP_METHOD_GET, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for admission validating webhook %s: %w", name, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get admission validating webhook %s: %w", name, err)
	}

	return responseUtils.ReadAndParseGenericResponse(resp), nil
}

// PatchAdmissionValidatingWebhook updates an admission validating webhook
func (c *Client) PatchAdmissionValidatingWebhook(name string, body map[string]interface{}) *response.GenericResponse {
	return c.sharedClient.PatchResource(
		admissionsEndpoints.PATCH_ADMISSION_VALIDATING_WEBHOOK,
		name,
		body,
	)
}

// DeleteAdmissionValidatingWebhook deletes an admission validating webhook
func (c *Client) DeleteAdmissionValidatingWebhook(name string) *response.GenericResponse {
	return c.sharedClient.DeleteResource(
		admissionsEndpoints.DELETE_ADMISSION_VALIDATING_WEBHOOK,
		name,
	)
}

// ValidateGrouper validates a grouper by name
func (c *Client) ValidateGrouper(name string) (*response.GenericResponse, error) {
	apiEndpoint := c.sharedClient.FormatEndpoint(string(admissionsEndpoints.VALIDATE_GROUPER), name)
	request := requestUtils.CreateGenericRequest(base.POST, base.ADMISSION_OPERATOR, base.V1, base.Endpoint(apiEndpoint))
	requestURL, err := request.GenerateURL()
	if err != nil {
		return nil, fmt.Errorf("failed to generate request URL for grouper validation %s: %w", name, err)
	}

	req, err := http.NewRequest(constants.HTTP_METHOD_POST, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for grouper validation %s: %w", name, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to validate grouper %s: %w", name, err)
	}

	return responseUtils.ReadAndParseGenericResponse(resp), nil
}
