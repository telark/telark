package roles

import (
	"github.com/plsyro/data/resources/finalizers"
	roleresource "github.com/plsyro/data/resources/role"
	resourcesshared "github.com/plsyro/data/resources/shared"
	"github.com/plsyro/rest/base"
	cleanupclient "github.com/plsyro/rest/clients/resources/cleanup"
	"github.com/plsyro/rest/clients/shared"
	"github.com/plsyro/rest/constants"
	eps "github.com/plsyro/rest/endpoints/resources/roles"
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

func (c *Client) CreateRole(role *roleresource.RoleAsResource) *response.GenericResponse {
	return c.Create(eps.CreateRole, role)
}

func (c *Client) GetRoleByID(id string) (*roleresource.RoleAsResource, error) {
	ep := shared.SubstituteEndpointWithParam(string(eps.GetRoleByID), constants.IDParam, id)
	return shared.GetTyped[roleresource.RoleAsResource](c.Client, ep)
}

func (c *Client) GetAllRoles() ([]*roleresource.RoleAsResource, error) {
	return shared.GetListTyped[*roleresource.RoleAsResource](c.Client, eps.GetAllRoles)
}

func (c *Client) GetRolesByUserID(userID string) ([]*roleresource.RoleAsResource, error) {
	ep := shared.SubstituteEndpointWithParam(string(eps.GetRolesByUserID), constants.UserIDParam, userID)
	return shared.GetListTyped[*roleresource.RoleAsResource](c.Client, ep)
}

func (c *Client) GetRolesByGroupID(groupID string) ([]*roleresource.RoleAsResource, error) {
	ep := shared.SubstituteEndpointWithParam(string(eps.GetRolesByGroupID), constants.GroupIDParam, groupID)
	return shared.GetListTyped[*roleresource.RoleAsResource](c.Client, ep)
}

func (c *Client) PatchRoleByID(id string, body map[string]any) *response.GenericResponse {
	ep := shared.SubstituteEndpointWithParam(string(eps.PatchRoleByID), constants.IDParam, id)
	return c.Update(ep, body)
}

func (c *Client) DeleteRoleByID(id string) *response.GenericResponse {
	ep := shared.SubstituteEndpointWithParam(string(eps.DeleteRoleByID), constants.IDParam, id)
	return c.Delete(ep)
}

func (c *Client) GetCleanupViewByID(id string) (*resourcesshared.CleanupView, error) {
	return cleanupclient.GetCleanupViewByID(c.Client, finalizers.ResourceTypeRoles, id)
}

func (c *Client) ListCleanupViews() ([]*resourcesshared.CleanupView, error) {
	return cleanupclient.ListCleanupViews(c.Client, finalizers.ResourceTypeRoles)
}

func (c *Client) AddFinalizer(id, name string) *response.GenericResponse {
	return cleanupclient.AddFinalizer(c.Client, finalizers.ResourceTypeRoles, id, name)
}

func (c *Client) RemoveFinalizer(id, name string) *response.GenericResponse {
	return cleanupclient.RemoveFinalizer(c.Client, finalizers.ResourceTypeRoles, id, name)
}
