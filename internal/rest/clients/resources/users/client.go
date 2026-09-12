package users

import (
	"fmt"
	"net/url"

	"github.com/telark/data/resources/finalizers"
	resourcesshared "github.com/telark/data/resources/shared"
	userresource "github.com/telark/data/resources/user"
	"github.com/telark/rest/base"
	cleanupclient "github.com/telark/rest/clients/resources/cleanup"
	"github.com/telark/rest/clients/shared"
	"github.com/telark/rest/constants"
	eps "github.com/telark/rest/endpoints/resources/users"
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

func (c *Client) CreateUser(user *userresource.UserAsResource) *response.GenericResponse {
	return c.Create(eps.CreateUser, user)
}

func (c *Client) GetUserByID(id string) (*userresource.UserAsResource, error) {
	byID := c.WithParams(map[string]string{constants.IDParam: id})
	return shared.GetTyped[userresource.UserAsResource](byID, eps.GetUserByID)
}

func (c *Client) GetUserByUsername(username string) (*userresource.UserAsResource, error) {
	params := map[string]string{constants.UsernameParam: username}
	return shared.GetTyped[userresource.UserAsResource](c.WithParams(params), eps.GetUserByUsername)
}

func (c *Client) GetUserByEmail(email string) (*userresource.UserAsResource, error) {
	params := map[string]string{constants.EmailParam: email}
	return shared.GetTyped[userresource.UserAsResource](c.WithParams(params), eps.GetUserByEmail)
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
	byID := c.WithParams(map[string]string{constants.IDParam: id})
	return byID.Update(eps.PatchUserByID, body)
}

func (c *Client) DeleteUserByID(id string) *response.GenericResponse {
	byID := c.WithParams(map[string]string{constants.IDParam: id})
	return byID.Delete(eps.DeleteUserByID)
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
