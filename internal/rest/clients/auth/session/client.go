package session

import (
	authdata "github.com/plsyro/data/auth"
	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/clients/shared"
	"github.com/plsyro/rest/constants"
	eps "github.com/plsyro/rest/endpoints/auth"
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

func (c *Client) CreateSessionByUser(userID string, session *authdata.UserSession) *response.GenericResponse {
	ep := shared.SubstituteEndpointWithParam(
		string(eps.CreateSessionByUser),
		constants.UserIDParam,
		userID,
	)
	return c.Create(ep, session)
}

func (c *Client) GetAllSessionsByUser(userID string) ([]*authdata.UserSession, error) {
	ep := shared.SubstituteEndpointWithParam(
		string(eps.GetAllSessionsByUser),
		constants.UserIDParam,
		userID,
	)
	return shared.GetListTyped[*authdata.UserSession](c.Client, ep)
}

func (c *Client) GetSessionByToken(token string) (*authdata.UserSession, error) {
	ep := shared.SubstituteEndpointWithParam(
		string(eps.GetSessionByToken),
		constants.TokenParam,
		token,
	)
	return shared.GetTyped[authdata.UserSession](c.Client, ep)
}

func (c *Client) PatchSessionByToken(token string, body map[string]any) *response.GenericResponse {
	ep := shared.SubstituteEndpointWithParam(
		string(eps.PatchSessionByToken),
		constants.TokenParam,
		token,
	)
	return c.Update(ep, body)
}

func (c *Client) DeleteSessionByToken(token string) *response.GenericResponse {
	ep := shared.SubstituteEndpointWithParam(
		string(eps.DeleteSessionByToken),
		constants.TokenParam,
		token,
	)
	return c.Delete(ep)
}
