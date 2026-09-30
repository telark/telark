package reports

import (
	"encoding/json"

	"github.com/telark/rest/base"
	"github.com/telark/rest/clients/shared"
	"github.com/telark/rest/constants"
	eps "github.com/telark/rest/endpoints/reports"
	"github.com/telark/rest/response"
)

type Client struct {
	*shared.Client
}

func NewClient() *Client {
	return &Client{Client: shared.New(base.Exporter)}
}

func NewClientWithConfig(cfg *shared.ClientConfig) *Client {
	return &Client{Client: shared.NewWithConfig(base.Exporter, cfg)}
}

func (c *Client) CreatePlanReport(req eps.CreatePlanReportRequest) *response.GenericResponse {
	return c.Create(eps.CreatePlanReport, req)
}

// PutLedger sends the caller's bytes verbatim: the ledger never goes through the reflection mapper.
func (c *Client) PutLedger(planID string, ledger json.RawMessage) *response.GenericResponse {
	byPlan := c.WithParams(map[string]string{constants.IDParam: planID})
	return shared.ExecuteRequestWithHeaders(byPlan, base.Update, eps.PutPlanReportLedger, ledger, nil)
}

func (c *Client) GetLedger(planID string) (json.RawMessage, error) {
	byPlan := c.WithParams(map[string]string{constants.IDParam: planID})
	ledger, err := shared.GetTyped[json.RawMessage](byPlan, eps.GetPlanReportLedger)
	if err != nil {
		return nil, err
	}
	return *ledger, nil
}
