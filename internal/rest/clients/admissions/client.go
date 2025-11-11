package admissions

import (
	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/clients/shared"
	"github.com/plsyro/rest/constants"
	eps "github.com/plsyro/rest/endpoints/admissions"
	"github.com/plsyro/rest/response"
)

type Client struct {
	*shared.Client
}

func NewClient() *Client {
	return &Client{
		Client: shared.New(base.Exporter),
	}
}

func (c *Client) CreateAdmissionValidatingWebhook(webhook any) *response.GenericResponse {
	return c.Create(eps.CreateAdmissionValidatingWebhook, webhook)
}

func (c *Client) GetAdmissionValidatingWebhook(name string) (*response.GenericResponse, error) {
	ep := shared.SubstituteEndpointWithParam(
		string(eps.GetAdmissionValidatingWebhook),
		constants.NameParam,
		name,
	)
	return c.Get(ep)
}

func (c *Client) PatchAdmissionValidatingWebhook(
	name string,
	body map[string]any,
) *response.GenericResponse {
	ep := shared.SubstituteEndpointWithParam(
		string(eps.PatchAdmissionValidatingWebhook),
		constants.NameParam,
		name,
	)
	return c.Update(ep, body)
}

func (c *Client) DeleteAdmissionValidatingWebhook(name string) *response.GenericResponse {
	ep := shared.SubstituteEndpointWithParam(
		string(eps.DeleteAdmissionValidatingWebhook),
		constants.NameParam,
		name,
	)
	return c.Delete(ep)
}

func (c *Client) ValidateGrouper(name string) (*response.GenericResponse, error) {
	ep := shared.SubstituteEndpointWithParam(
		string(eps.ValidateGrouper),
		constants.NameParam,
		name,
	)
	return c.Post(ep)
}
