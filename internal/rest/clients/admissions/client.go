package admissions

import (
	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/clients/shared"
	"github.com/plsyro/rest-pkg/constants"
	admissionsEndpoints "github.com/plsyro/rest-pkg/endpoints/admissions"
	response "github.com/plsyro/rest-pkg/response"
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
	return c.GetResourceByName(admissionsEndpoints.GET_ADMISSION_VALIDATING_WEBHOOK, name)
}

func (c *Client) PatchAdmissionValidatingWebhook(name string, body map[string]interface{}) *response.GenericResponse {
	return c.PatchResource(admissionsEndpoints.PATCH_ADMISSION_VALIDATING_WEBHOOK, name, body)
}

func (c *Client) DeleteAdmissionValidatingWebhook(name string) *response.GenericResponse {
	return c.DeleteResource(admissionsEndpoints.DELETE_ADMISSION_VALIDATING_WEBHOOK, name)
}

func (c *Client) ValidateGrouper(name string) (*response.GenericResponse, error) {
	apiEndpoint := c.FormatEndpoint(string(admissionsEndpoints.VALIDATE_GROUPER), name)
	return c.PostResourceNoPayload(base.Endpoint(apiEndpoint))
}
