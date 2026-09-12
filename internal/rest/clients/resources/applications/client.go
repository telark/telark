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
	return shared.GetTyped[appresource.Application](byName(c.Client, name), eps.GetApplicationByName)
}

func (c *Client) GetAllApplications() ([]*appresource.Application, error) {
	return shared.GetListTyped[*appresource.Application](c.Client, eps.GetAllApplications)
}

func (c *Client) PatchApplicationByName(name string, body map[string]any) *response.GenericResponse {
	return byName(c.Client, name).Update(eps.PatchApplicationByName, body)
}

func (c *Client) DeleteApplicationByName(name string) *response.GenericResponse {
	return byName(c.Client, name).Delete(eps.DeleteApplicationByName)
}

func (*Client) CleanupApplicationByName(name string) *response.GenericResponse {
	discoveryClient := shared.New(base.Discovery)
	return byName(discoveryClient, name).Delete(eps.CleanupApplication)
}

func byName(c *shared.Client, name string) *shared.Client {
	return c.WithParams(map[string]string{constants.NameParam: name})
}
