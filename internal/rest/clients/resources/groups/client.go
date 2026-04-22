package groups

import (
	groupresource "github.com/plsyro/data/resources/group"
	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/clients/shared"
	"github.com/plsyro/rest/constants"
	eps "github.com/plsyro/rest/endpoints/resources/groups"
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

func (c *Client) CreateGroup(group *groupresource.GroupAsResource) *response.GenericResponse {
	return c.Create(eps.CreateGroup, group)
}

func (c *Client) GetGroupByID(id string) (*groupresource.GroupAsResource, error) {
	ep := shared.SubstituteEndpointWithParam(
		string(eps.GetGroupByID),
		constants.IDParam,
		id,
	)
	return shared.GetTyped[groupresource.GroupAsResource](c.Client, ep)
}

func (c *Client) GetAllGroups() ([]*groupresource.GroupAsResource, error) {
	return shared.GetListTyped[*groupresource.GroupAsResource](c.Client, eps.GetAllGroups)
}

func (c *Client) PatchGroupByID(id string, body map[string]any) *response.GenericResponse {
	ep := shared.SubstituteEndpointWithParam(
		string(eps.PatchGroupByID),
		constants.IDParam,
		id,
	)
	return c.Update(ep, body)
}

func (c *Client) DeleteGroupByID(id string) *response.GenericResponse {
	ep := shared.SubstituteEndpointWithParam(
		string(eps.DeleteGroupByID),
		constants.IDParam,
		id,
	)
	return c.Delete(ep)
}
