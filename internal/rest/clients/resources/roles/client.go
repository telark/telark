package roles

import (
	"github.com/telark/data/resources/finalizers"
	roleresource "github.com/telark/data/resources/role"
	resourcesshared "github.com/telark/data/resources/shared"
	"github.com/telark/rest/base"
	cleanupclient "github.com/telark/rest/clients/resources/cleanup"
	"github.com/telark/rest/clients/shared"
	"github.com/telark/rest/constants"
	eps "github.com/telark/rest/endpoints/resources/roles"
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

func (c *Client) CreateRole(role *roleresource.RoleAsResource) *response.GenericResponse {
	return c.Create(eps.CreateRole, role)
}

func (c *Client) GetRoleByID(id string) (*roleresource.RoleAsResource, error) {
	byID := c.WithParams(map[string]string{constants.IDParam: id})
	return shared.GetTyped[roleresource.RoleAsResource](byID, eps.GetRoleByID)
}

func (c *Client) GetAllRoles() ([]*roleresource.RoleAsResource, error) {
	return shared.GetListTyped[*roleresource.RoleAsResource](c.Client, eps.GetAllRoles)
}

func (c *Client) GetRolesByUserID(userID string) ([]*roleresource.RoleAsResource, error) {
	params := map[string]string{constants.UserIDParam: userID}
	return shared.GetListTyped[*roleresource.RoleAsResource](c.WithParams(params), eps.GetRolesByUserID)
}

func (c *Client) GetRolesByGroupID(groupID string) ([]*roleresource.RoleAsResource, error) {
	params := map[string]string{constants.GroupIDParam: groupID}
	return shared.GetListTyped[*roleresource.RoleAsResource](c.WithParams(params), eps.GetRolesByGroupID)
}

func (c *Client) PatchRoleByID(id string, body map[string]any) *response.GenericResponse {
	byID := c.WithParams(map[string]string{constants.IDParam: id})
	return byID.Update(eps.PatchRoleByID, body)
}

func (c *Client) DeleteRoleByID(id string) *response.GenericResponse {
	byID := c.WithParams(map[string]string{constants.IDParam: id})
	return byID.Delete(eps.DeleteRoleByID)
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
