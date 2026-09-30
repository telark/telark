package accessroles

import (
	"github.com/telark/telark/internal/data/resources/finalizers"
	roleresource "github.com/telark/telark/internal/data/resources/role"
	resourcesshared "github.com/telark/telark/internal/data/resources/shared"
	"github.com/telark/telark/internal/rest/base"
	cleanupclient "github.com/telark/telark/internal/rest/clients/cleanup"
	"github.com/telark/telark/internal/rest/clients/shared"
	"github.com/telark/telark/internal/rest/constants"
	eps "github.com/telark/telark/internal/rest/endpoints/accessroles"
	"github.com/telark/telark/internal/rest/response"
)

type Client struct {
	*shared.Client
}

func NewClient() *Client {
	return &Client{
		Client: shared.New(base.Exporter),
	}
}

func (c *Client) CreateAccessRole(role *roleresource.AccessRole) *response.GenericResponse {
	return c.Create(eps.CreateAccessRole, role)
}

func (c *Client) GetAccessRoleByID(id string) (*roleresource.AccessRole, error) {
	byID := c.WithParams(map[string]string{constants.IDParam: id})
	return shared.GetTypedNoCache[roleresource.AccessRole](byID, eps.GetAccessRoleByID)
}

func (c *Client) GetAllAccessRoles() ([]*roleresource.AccessRole, error) {
	return shared.GetListTyped[*roleresource.AccessRole](c.Client, eps.GetAllAccessRoles)
}

func (c *Client) PatchAccessRoleByID(id string, body map[string]any) *response.GenericResponse {
	byID := c.WithParams(map[string]string{constants.IDParam: id})
	return byID.Update(eps.PatchAccessRoleByID, body)
}

func (c *Client) DeleteAccessRoleByID(id string) *response.GenericResponse {
	byID := c.WithParams(map[string]string{constants.IDParam: id})
	return byID.Delete(eps.DeleteAccessRoleByID)
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
