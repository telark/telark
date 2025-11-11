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
	endpoint := base.Endpoint(shared.SubstituteEndpointWithParam(
		string(eps.GetUserByID),
		"{id}",
		id,
	))
	return shared.GetTyped[userresource.UserAsResource](c.Client, endpoint)
}

func (c *Client) GetUserByUsername(username string) (*userresource.UserAsResource, error) {
	endpoint := base.Endpoint(shared.SubstituteEndpointWithParam(
		string(eps.GetUserByUsername),
		"{username}",
		username,
	))
	return shared.GetTyped[userresource.UserAsResource](c.Client, endpoint)
}

func (c *Client) GetAllUsers() ([]*userresource.UserAsResource, error) {
	return shared.GetListTyped[*userresource.UserAsResource](c.Client, eps.GetAllUsers)
}

func (c *Client) PatchUserByID(id string, body map[string]any) *response.GenericResponse {
	endpoint := base.Endpoint(shared.SubstituteEndpointWithParam(
		string(eps.PatchUserByID),
		"{id}",
		id,
	))
	return c.Update(endpoint, body)
}

func (c *Client) DeleteUserByID(id string) *response.GenericResponse {
	endpoint := base.Endpoint(shared.SubstituteEndpointWithParam(
		string(eps.DeleteUserByID),
		"{id}",
		id,
	))
	return c.Delete(endpoint)
}
