package admissions

import (
	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/clients/shared"
	admissionsEndpoints "github.com/plsyro/rest-pkg/endpoints/admissions"
	response "github.com/plsyro/rest-pkg/response"
)

type Client struct {
	*shared.Client
}

func NewClient() *Client {
	return &Client{
		Client: shared.New(base.ADMISSION_OPERATOR),
	}
}

func (c *Client) CreateAdmissionValidatingWebhook(webhook interface{}) *response.GenericResponse {
	return c.Create(admissionsEndpoints.CREATE_ADMISSION_VALIDATING_WEBHOOK, webhook)
}

func (c *Client) GetAdmissionValidatingWebhook(name string) (*response.GenericResponse, error) {
	return c.Get(admissionsEndpoints.GET_ADMISSION_VALIDATING_WEBHOOK, name)
}

func (c *Client) PatchAdmissionValidatingWebhook(name string, body map[string]interface{}) *response.GenericResponse {
	return c.Update(admissionsEndpoints.PATCH_ADMISSION_VALIDATING_WEBHOOK, name, body)
}

func (c *Client) DeleteAdmissionValidatingWebhook(name string) *response.GenericResponse {
	return c.Delete(admissionsEndpoints.DELETE_ADMISSION_VALIDATING_WEBHOOK, name)
}

func (c *Client) ValidateGrouper(name string) (*response.GenericResponse, error) {
	apiEndpoint := c.FormatEndpoint(string(admissionsEndpoints.VALIDATE_GROUPER), name)
	return c.Post(base.Endpoint(apiEndpoint))
}
