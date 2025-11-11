package challenge

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

func (c *Client) CreateChallengeByUser(userID string, challenge *authdata.AuthChallenge) *response.GenericResponse {
	endpoint := base.Endpoint(shared.SubstituteEndpointParam(string(eps.CreateChallengeByUser), "{userId}", userID))
	return c.Create(endpoint, challenge)
}

func (c *Client) GetChallengeByUser(userID string) (*authdata.AuthChallenge, error) {
	endpoint := base.Endpoint(shared.SubstituteEndpointParam(string(eps.GetChallengeByUser), "{userId}", userID))
	return shared.GetTyped[authdata.AuthChallenge](c.Client, endpoint, "")
}

func (c *Client) DeleteChallengeByUser(userID string) *response.GenericResponse {
	endpoint := base.Endpoint(shared.SubstituteEndpointParam(string(eps.DeleteChallengeByUser), "{userId}", userID))
	return c.DeleteNoParams(endpoint)
}
