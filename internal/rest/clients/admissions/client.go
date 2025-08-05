package admissions

import (
	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/clients/shared"
	admissionsEndpoints "github.com/plsyro/rest/endpoints/admissions"
	response "github.com/plsyro/rest/response"
)

type Client struct {
	*shared.Client
}

func NewClient() *Client {
	return &Client{
		Client: shared.New(base.AdmissionOperator),
	}
}

func (c *Client) CreateAdmissionValidatingWebhook(webhook interface{}) *response.GenericResponse {
	return c.Create(admissionsEndpoints.CreateAdmissionValidatingWebhook, webhook)
}

func (c *Client) GetAdmissionValidatingWebhook(name string) (*response.GenericResponse, error) {
	return c.Get(admissionsEndpoints.GetAdmissionValidatingWebhook, name)
}

func (c *Client) PatchAdmissionValidatingWebhook(name string, body map[string]interface{}) *response.GenericResponse {
	return c.Update(admissionsEndpoints.PatchAdmissionValidatingWebhook, name, body)
}

func (c *Client) DeleteAdmissionValidatingWebhook(name string) *response.GenericResponse {
	return c.Delete(admissionsEndpoints.DeleteAdmissionValidatingWebhook, name)
}

func (c *Client) ValidateGrouper(name string) (*response.GenericResponse, error) {
	apiEndpoint := c.FormatEndpoint(string(admissionsEndpoints.ValidateGrouper), name)
	return c.Post(base.Endpoint(apiEndpoint))
}
