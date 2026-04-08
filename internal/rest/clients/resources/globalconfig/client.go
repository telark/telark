package globalconfig

import (
	globalconfigresource "github.com/plsyro/data/resources/globalconfig"
	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/clients/shared"
	eps "github.com/plsyro/rest/endpoints/resources/globalconfig"
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

func NewClientWithConfig(cfg *shared.ClientConfig) *Client {
	return &Client{
		Client: shared.NewWithConfig(base.Exporter, cfg),
	}
}

func (c *Client) GetGlobalConfig() (*globalconfigresource.GlobalConfig, error) {
	return shared.GetTyped[globalconfigresource.GlobalConfig](c.Client, eps.GetGlobalConfig)
}

func (c *Client) PatchGlobalConfig(body map[string]any) *response.GenericResponse {
	return c.Update(eps.PatchGlobalConfig, body)
}
