package challenge

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

func (c *Client) CreateChallengeByUser(userID string, challenge *authdata.AuthChallenge) *response.GenericResponse {
	ep := base.Endpoint(shared.SubstituteEndpointWithParam(
		string(eps.CreateChallengeByUser),
		constants.UserIDParam,
		userID,
	))
	return c.Create(ep, challenge)
}

func (c *Client) GetChallengeByUser(userID string) (*authdata.AuthChallenge, error) {
	ep := base.Endpoint(shared.SubstituteEndpointWithParam(
		string(eps.GetChallengeByUser),
		constants.UserIDParam,
		userID,
	))
	return shared.GetTyped[authdata.AuthChallenge](c.Client, ep)
}

func (c *Client) DeleteChallengeByUser(userID string) *response.GenericResponse {
	ep := base.Endpoint(shared.SubstituteEndpointWithParam(
		string(eps.DeleteChallengeByUser),
		constants.UserIDParam,
		userID,
	))
	return c.Delete(ep)
}
