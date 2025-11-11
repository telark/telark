package grouper

import (
	grouperresource "github.com/plsyro/data/resources/grouper"
	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/clients/shared"
	"github.com/plsyro/rest/constants"
	eps "github.com/plsyro/rest/endpoints/resources/groupers"
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

func (c *Client) CreateGrouper(
	grouper *grouperresource.GrouperAsResource,
) *response.GenericResponse {
	return c.Create(eps.CreateGrouper, grouper)
}

func (c *Client) GetGrouperByName(name string) (*grouperresource.GrouperAsResource, error) {
	ep := base.Endpoint(shared.SubstituteEndpointWithParam(string(eps.GetGrouper),
		constants.NameParam,
		name,
	))
	return shared.GetTyped[grouperresource.GrouperAsResource](c.Client, ep)
}

func (c *Client) GetAllGroupers() ([]*grouperresource.GrouperAsResource, error) {
	return shared.GetListTyped[*grouperresource.GrouperAsResource](c.Client,
		eps.GetAllGroupers)
}

func (c *Client) PatchGrouper(name string, body map[string]any) *response.GenericResponse {
	ep := base.Endpoint(shared.SubstituteEndpointWithParam(
		string(eps.PatchGrouper),
		constants.NameParam,
		name,
	))
	return c.Update(ep, body)
}

func (c *Client) DeleteGrouper(name string) *response.GenericResponse {
	ep := base.Endpoint(shared.SubstituteEndpointWithParam(
		string(eps.DeleteGrouper),
		constants.NameParam,
		name,
	))
	return c.Delete(ep)
}
