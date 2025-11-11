package users

import (
	userresource "github.com/plsyro/data/resources/user"
	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/clients/shared"
	eps "github.com/plsyro/rest/endpoints/resources/users"
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

func (c *Client) CreateUser(user *userresource.UserAsResource) *response.GenericResponse {
	return c.Create(eps.CreateUser, user)
}

func (c *Client) GetUserByID(id string) (*userresource.UserAsResource, error) {
	return shared.GetTyped[userresource.UserAsResource](c.Client, eps.GetUserByID, id)
}

func (c *Client) GetUserByUsername(username string) (*userresource.UserAsResource, error) {
	return shared.GetTyped[userresource.UserAsResource](c.Client, eps.GetUserByUsername, username)
}

func (c *Client) GetAllUsers() ([]*userresource.UserAsResource, error) {
	return shared.GetListTyped[*userresource.UserAsResource](c.Client, eps.GetAllUsers)
}

func (c *Client) PatchUserByID(id string, body map[string]any) *response.GenericResponse {
	return c.Update(eps.PatchUserByID, id, body)
}

func (c *Client) DeleteUserByID(id string) *response.GenericResponse {
	return c.Delete(eps.DeleteUserByID, id)
}
