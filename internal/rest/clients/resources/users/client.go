package users

import (
	"fmt"
	"net/url"

	"github.com/plsyro/data/resources/finalizers"
	resourcesshared "github.com/plsyro/data/resources/shared"
	userresource "github.com/plsyro/data/resources/user"
	"github.com/plsyro/rest/base"
	cleanupclient "github.com/plsyro/rest/clients/resources/cleanup"
	"github.com/plsyro/rest/clients/shared"
	"github.com/plsyro/rest/constants"
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
	ep := shared.SubstituteEndpointWithParam(string(eps.GetUserByID), constants.IDParam, id)
	return shared.GetTyped[userresource.UserAsResource](c.Client, ep)
}

func (c *Client) GetUserByUsername(username string) (*userresource.UserAsResource, error) {
	ep := shared.SubstituteEndpointWithParam(string(eps.GetUserByUsername), constants.UsernameParam, username)
	return shared.GetTyped[userresource.UserAsResource](c.Client, ep)
}

func (c *Client) GetUserByEmail(email string) (*userresource.UserAsResource, error) {
	ep := shared.SubstituteEndpointWithParam(string(eps.GetUserByEmail), constants.EmailParam, email)
	return shared.GetTyped[userresource.UserAsResource](c.Client, ep)
}

func (c *Client) GetUserByIdentity(provider, issuer, subject string) (*userresource.UserAsResource, error) {
	ep := base.Endpoint(fmt.Sprintf("%s?provider=%s&issuer=%s&subject=%s",
		eps.GetUserByIdentity,
		url.QueryEscape(provider),
		url.QueryEscape(issuer),
		url.QueryEscape(subject),
	))
	return shared.GetTyped[userresource.UserAsResource](c.Client, ep)
}

func (c *Client) GetAllUsers() ([]*userresource.UserAsResource, error) {
	return shared.GetListTyped[*userresource.UserAsResource](c.Client, eps.GetAllUsers)
}

func (c *Client) PatchUserByID(id string, body map[string]any) *response.GenericResponse {
	ep := shared.SubstituteEndpointWithParam(string(eps.PatchUserByID), constants.IDParam, id)
	return c.Update(ep, body)
}

func (c *Client) DeleteUserByID(id string) *response.GenericResponse {
	ep := shared.SubstituteEndpointWithParam(string(eps.DeleteUserByID), constants.IDParam, id)
	return c.Delete(ep)
}

func (c *Client) GetCleanupViewByID(id string) (*resourcesshared.CleanupView, error) {
	return cleanupclient.GetCleanupViewByID(c.Client, finalizers.ResourceTypeUsers, id)
}

func (c *Client) ListCleanupViews() ([]*resourcesshared.CleanupView, error) {
	return cleanupclient.ListCleanupViews(c.Client, finalizers.ResourceTypeUsers)
}

func (c *Client) AddFinalizer(id, name string) *response.GenericResponse {
	return cleanupclient.AddFinalizer(c.Client, finalizers.ResourceTypeUsers, id, name)
}

func (c *Client) RemoveFinalizer(id, name string) *response.GenericResponse {
	return cleanupclient.RemoveFinalizer(c.Client, finalizers.ResourceTypeUsers, id, name)
}
