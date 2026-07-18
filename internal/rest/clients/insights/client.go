package insights

import (
	insightsdata "github.com/telark/data/insights"
	"github.com/telark/rest/base"
	"github.com/telark/rest/clients/shared"
	eps "github.com/telark/rest/endpoints/insights"
	"github.com/telark/rest/response"
)

type Client struct {
	*shared.Client
}

func NewClient() *Client {
	return &Client{
		Client: shared.New(base.Enrichment),
	}
}

func NewClientWithConfig(cfg *shared.ClientConfig) *Client {
	return &Client{
		Client: shared.NewWithConfig(base.Enrichment, cfg),
	}
}

func (c *Client) DispatchApplications(signals []insightsdata.Signal) *response.GenericResponse {
	return c.Create(eps.Applications, insightsdata.DispatchRequest{Items: signals})
}
