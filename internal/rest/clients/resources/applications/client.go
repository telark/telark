package applications

import (
	appresource "github.com/telark/data/resources/application"
	"github.com/telark/rest/base"
	"github.com/telark/rest/clients/shared"
	"github.com/telark/rest/constants"
	eps "github.com/telark/rest/endpoints/resources/applications"
	"github.com/telark/rest/response"
)

type Client struct {
	*shared.Client
}

func NewClient() *Client {
	return &Client{
		Client: shared.New(base.Exporter),
	}
}

func NewClientWithConfig(cfg *shared.ClientConfig) *Client {
	return &Client{
		Client: shared.NewWithConfig(base.Exporter, cfg),
	}
}

func (c *Client) CreateApplication(app *appresource.Application) *response.GenericResponse {
	return c.Create(eps.CreateApplication, app)
}

func (c *Client) GetApplicationByName(name string) (*appresource.Application, error) {
	ep := shared.SubstituteEndpointWithParam(
		string(eps.GetApplicationByName),
		constants.NameParam,
		name,
	)
	return shared.GetTyped[appresource.Application](c.Client, ep)
}

func (c *Client) GetAllApplications() ([]*appresource.Application, error) {
	return shared.GetListTyped[*appresource.Application](c.Client, eps.GetAllApplications)
}

func (c *Client) PatchApplicationByName(name string, body map[string]any) *response.GenericResponse {
	ep := shared.SubstituteEndpointWithParam(
		string(eps.PatchApplicationByName),
		constants.NameParam,
		name,
	)
	return c.Update(ep, body)
}

func (c *Client) DeleteApplicationByName(name string) *response.GenericResponse {
	ep := shared.SubstituteEndpointWithParam(
		string(eps.DeleteApplicationByName),
		constants.NameParam,
		name,
	)
	return c.Delete(ep)
}

func (*Client) CleanupApplicationByName(name string) *response.GenericResponse {
	ep := shared.SubstituteEndpointWithParam(
		string(eps.CleanupApplication),
		constants.NameParam,
		name,
	)
	discoveryClient := shared.New(base.Discovery)
	return discoveryClient.Delete(ep)
}
