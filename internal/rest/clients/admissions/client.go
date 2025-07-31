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

type Client struct {
	*shared.BaseResourceClient
}

func NewClient() *Client {
	return &Client{
		BaseResourceClient: shared.NewBaseResourceClient(base.ADMISSION_OPERATOR, constants.RESOURCE_TYPE_ADMISSION_WEBHOOK),
	}
}

func (c *Client) CreateAdmissionValidatingWebhook(webhook interface{}) *response.GenericResponse {
	return c.CreateResource(admissionsEndpoints.CREATE_ADMISSION_VALIDATING_WEBHOOK, webhook)
}

func (c *Client) GetAdmissionValidatingWebhook(name string) (*response.GenericResponse, error) {
	apiEndpoint := c.FormatEndpoint(string(admissionsEndpoints.GET_ADMISSION_VALIDATING_WEBHOOK), name)
	request := requestUtils.CreateGenericRequest(base.GET, base.ADMISSION_OPERATOR, base.V1, base.Endpoint(apiEndpoint))
	requestURL, err := request.GenerateURL()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GENERATE_REQUEST_URL_FOR), constants.RESOURCE_TYPE_ADMISSION_WEBHOOK, name, err)
	}

	req, err := http.NewRequest(constants.HTTP_METHOD_GET, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_CREATE_REQUEST), constants.RESOURCE_TYPE_ADMISSION_WEBHOOK, name, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GET_RESOURCE), constants.RESOURCE_TYPE_ADMISSION_WEBHOOK, name, err)
	}

	return responseUtils.ReadAndParseGenericResponse(resp), nil
}

func (c *Client) PatchAdmissionValidatingWebhook(name string, body map[string]interface{}) *response.GenericResponse {
	return c.PatchResource(admissionsEndpoints.PATCH_ADMISSION_VALIDATING_WEBHOOK, name, body)
}

func (c *Client) DeleteAdmissionValidatingWebhook(name string) *response.GenericResponse {
	return c.DeleteResource(admissionsEndpoints.DELETE_ADMISSION_VALIDATING_WEBHOOK, name)
}

func (c *Client) ValidateGrouper(name string) (*response.GenericResponse, error) {
	apiEndpoint := c.FormatEndpoint(string(admissionsEndpoints.VALIDATE_GROUPER), name)
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
