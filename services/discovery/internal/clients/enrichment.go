package clients

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	insightsdata "github.com/telark/data/insights"
	"github.com/telark/discovery/internal/circuitbreaker"
	"github.com/telark/discovery/internal/constants"
	insightsclient "github.com/telark/rest/clients/insights"
	"github.com/telark/rest/clients/shared"
)

const dispatchInsightsTimeout = 10 * time.Second

type EnrichmentClient struct {
	client *insightsclient.Client
}

func NewEnrichmentClient() *EnrichmentClient {
	cfg := &shared.ClientConfig{Timeout: dispatchInsightsTimeout}
	return &EnrichmentClient{
		client: insightsclient.NewClientWithConfig(cfg),
	}
}

// DispatchApplications sends the whole scope in one call. It returns once the
// batch is queued, not once the insights exist — the model work happens in the
// enrichment workers and is read back from the cache later.
func (c *EnrichmentClient) DispatchApplications(signals []insightsdata.Signal) error {
	return circuitbreaker.ExecuteEnrichment(func() error {
		resp := c.client.DispatchApplications(signals)
		if resp == nil {
			return errors.New(string(constants.ErrDispatchInsightsNilResponse))
		}
		if resp.Status == http.StatusOK {
			return nil
		}
		return classifyStatus(resp.Status, fmt.Errorf(
			string(constants.ErrDispatchInsightsFailed), resp.Status, resp.Message,
		))
	})
}
