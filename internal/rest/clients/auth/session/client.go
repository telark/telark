package session

import (
	authdata "github.com/telark/data/auth"
	"github.com/telark/rest/base"
	"github.com/telark/rest/clients/shared"
	"github.com/telark/rest/constants"
	eps "github.com/telark/rest/endpoints/auth"
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

func (c *Client) withUser(userID string) *shared.Client {
	return c.WithParams(map[string]string{constants.UserIDParam: userID})
}

func (c *Client) withToken(token string) *shared.Client {
	return c.WithParams(map[string]string{constants.TokenParam: token})
}

func (c *Client) CreateSessionByUser(userID string, session *authdata.UserSession) *response.GenericResponse {
	return c.withUser(userID).Create(eps.CreateSessionByUser, session)
}

func (c *Client) GetAllSessionsByUser(userID string) ([]*authdata.UserSession, error) {
	return shared.GetListTyped[*authdata.UserSession](c.withUser(userID), eps.GetAllSessionsByUser)
}

func (c *Client) GetSessionByToken(token string) (*authdata.UserSession, error) {
	return shared.GetTyped[authdata.UserSession](c.withToken(token), eps.GetSessionByToken)
}

func (c *Client) PatchSessionByToken(token string, body map[string]any) *response.GenericResponse {
	return c.withToken(token).Update(eps.PatchSessionByToken, body)
}

func (c *Client) DeleteSessionByToken(token string) *response.GenericResponse {
	return c.withToken(token).Delete(eps.DeleteSessionByToken)
}
