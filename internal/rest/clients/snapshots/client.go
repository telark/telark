package snapshots

import (
	"net/url"

	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/clients/shared"
	"github.com/plsyro/rest/constants"
	eps "github.com/plsyro/rest/endpoints/snapshots"
	"github.com/plsyro/rest/response"
)

type Client struct {
	*shared.Client
}

type CreateSnapshotPayload struct {
	ID       string `json:"id"`
	Scope    string `json:"scope"`
	Version  int    `json:"version"`
	Manifest any    `json:"manifest"`
}

func NewClient() *Client {
	return &Client{
		Client: shared.New(base.Exporter),
	}
}

func (c *Client) CreateSnapshot(payload *CreateSnapshotPayload) *response.GenericResponse {
	return c.Create(eps.CreateSnapshot, payload)
}

func (c *Client) GetSnapshot(id string, scope string) (*map[string]any, error) {
	ep := shared.SubstituteEndpointWithParam(
		string(eps.GetSnapshot),
		constants.IDParam,
		id,
	)

	withQuery := base.Endpoint(string(ep) + "?scope=" + url.QueryEscape(scope))
	return shared.GetTyped[map[string]any](c.Client, withQuery)
}
