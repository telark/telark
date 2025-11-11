package session

import (
	authdata "github.com/plsyro/data/auth"
	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/clients/shared"
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
	endpoint := base.Endpoint(shared.SubstituteEndpointWithParam(string(eps.CreateSessionByUser), "{userId}", userID))
	return c.Create(endpoint, session)
}

func (c *Client) GetAllSessionsByUser(userID string) ([]*authdata.UserSession, error) {
	endpoint := base.Endpoint(shared.SubstituteEndpointWithParam(string(eps.GetAllSessionsByUser), "{userId}", userID))
	return shared.GetListTyped[*authdata.UserSession](c.Client, endpoint)
}

func (c *Client) GetSessionByToken(token string) (*authdata.UserSession, error) {
	endpoint := base.Endpoint(shared.SubstituteEndpointWithParam(string(eps.GetSessionByToken), "{token}", token))
	return shared.GetTyped[authdata.UserSession](c.Client, endpoint)
}

func (c *Client) PatchSessionByToken(token string, body map[string]any) *response.GenericResponse {
	endpoint := base.Endpoint(shared.SubstituteEndpointWithParam(string(eps.PatchSessionByToken), "{token}", token))
	return c.Update(endpoint, token, body)
}

func (c *Client) DeleteSessionByToken(token string) *response.GenericResponse {
	endpoint := base.Endpoint(shared.SubstituteEndpointWithParam(string(eps.DeleteSessionByToken), "{token}", token))
	return c.DeleteNoParams(endpoint)
}
