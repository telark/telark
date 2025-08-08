package analyze

import (
	"fmt"
	"net/http"

	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/clients/shared"
	"github.com/plsyro/rest/constants"
	eps "github.com/plsyro/rest/endpoints/analyze"
)

type Client struct {
	*shared.Client
}

func NewClient() *Client {
	return &Client{
		Client: shared.New(base.Configurator),
	}
}

func (c *Client) StartAnalyze() error {
	responses, err := c.PostAndParseGenericResponses(eps.Startanalyze)
	if err != nil {
		return err
	}

	for _, resp := range responses {
		if resp.Status != http.StatusAccepted {
			return fmt.Errorf(string(constants.ErrUnexpectedStatus), resp.Status, resp.Message)
		}
	}

	return nil
}
