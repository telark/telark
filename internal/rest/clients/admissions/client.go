package admissions

import (
	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/clients/shared"
	eps "github.com/plsyro/rest/endpoints/admissions"
	"github.com/plsyro/rest/response"
)

type Client struct {
	*shared.Client
}

func NewClient() *Client {
	return &Client{
		Client: shared.New(base.AdmissionOperator),
	}
}

func (c *Client) CreateAdmissionValidatingWebhook(webhook any) *response.GenericResponse {
	return c.Create(eps.CreateAdmissionValidatingWebhook, webhook)
}

func (c *Client) GetAdmissionValidatingWebhook(name string) (*response.GenericResponse, error) {
	return c.Get(eps.GetAdmissionValidatingWebhook, name)
}

func (c *Client) PatchAdmissionValidatingWebhook(
	name string,
	body map[string]any,
) *response.GenericResponse {
	return c.Update(eps.PatchAdmissionValidatingWebhook, name, body)
}

func (c *Client) DeleteAdmissionValidatingWebhook(name string) *response.GenericResponse {
	return c.Delete(eps.DeleteAdmissionValidatingWebhook, name)
}

func (c *Client) ValidateGrouper(name string) (*response.GenericResponse, error) {
	apiEndpoint := c.FormatEndpoint(string(eps.ValidateGrouper), name)
	return c.Post(base.Endpoint(apiEndpoint))
}
