package groups

import (
	"github.com/telark/data/resources/finalizers"
	groupresource "github.com/telark/data/resources/group"
	resourcesshared "github.com/telark/data/resources/shared"
	"github.com/telark/rest/base"
	cleanupclient "github.com/telark/rest/clients/resources/cleanup"
	"github.com/telark/rest/clients/shared"
	"github.com/telark/rest/constants"
	eps "github.com/telark/rest/endpoints/resources/groups"
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

func (c *Client) CreateGroup(group *groupresource.GroupAsResource) *response.GenericResponse {
	return c.Create(eps.CreateGroup, group)
}

func (c *Client) GetGroupByID(id string) (*groupresource.GroupAsResource, error) {
	ep := shared.SubstituteEndpointWithParam(string(eps.GetGroupByID), constants.IDParam, id)
	return shared.GetTyped[groupresource.GroupAsResource](c.Client, ep)
}

func (c *Client) GetAllGroups() ([]*groupresource.GroupAsResource, error) {
	return shared.GetListTyped[*groupresource.GroupAsResource](c.Client, eps.GetAllGroups)
}

func (c *Client) PatchGroupByID(id string, body map[string]any) *response.GenericResponse {
	ep := shared.SubstituteEndpointWithParam(string(eps.PatchGroupByID), constants.IDParam, id)
	return c.Update(ep, body)
}

func (c *Client) DeleteGroupByID(id string) *response.GenericResponse {
	ep := shared.SubstituteEndpointWithParam(string(eps.DeleteGroupByID), constants.IDParam, id)
	return c.Delete(ep)
}

func (c *Client) GetCleanupViewByID(id string) (*resourcesshared.CleanupView, error) {
	return cleanupclient.GetCleanupViewByID(c.Client, finalizers.ResourceTypeGroups, id)
}

func (c *Client) ListCleanupViews() ([]*resourcesshared.CleanupView, error) {
	return cleanupclient.ListCleanupViews(c.Client, finalizers.ResourceTypeGroups)
}

func (c *Client) AddFinalizer(id, name string) *response.GenericResponse {
	return cleanupclient.AddFinalizer(c.Client, finalizers.ResourceTypeGroups, id, name)
}

func (c *Client) RemoveFinalizer(id, name string) *response.GenericResponse {
	return cleanupclient.RemoveFinalizer(c.Client, finalizers.ResourceTypeGroups, id, name)
}
