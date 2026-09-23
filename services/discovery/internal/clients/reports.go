package clients

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection/validation"
	reportsclient "github.com/telark/rest/clients/reports"
	"github.com/telark/rest/clients/shared"
	reportseps "github.com/telark/rest/endpoints/reports"
	"github.com/telark/rest/response"
)

// ReportClient deliberately bypasses the exporter circuit breaker: report calls must never
// take its half-open probe slot nor record results on the breaker that guards CreateSnapshot.
// The HTTP timeout is the capture bound.
type ReportClient struct {
	client *reportsclient.Client
}

func NewReportClient() *ReportClient {
	cfg := &shared.ClientConfig{Timeout: constants.DefaultReportCaptureTimeout}
	return &ReportClient{client: reportsclient.NewClientWithConfig(cfg)}
}

func (c *ReportClient) HTTPClient() *http.Client {
	return c.client.GetHTTPClient()
}

func ClassifyReportResponse(resp *response.GenericResponse) error {
	if resp == nil {
		return wrapExporter(errors.New(string(constants.ErrReportNilResponse)))
	}
	switch resp.Status {
	case http.StatusOK:
		return nil
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge:
		return validation.Invalidf(string(constants.ErrReportRejected), resp.Message)
	default:
		return wrapExporter(fmt.Errorf(string(constants.ErrRestCallFailed), resp.Status, resp.Message))
	}
}

func (c *ReportClient) Create(req reportseps.CreatePlanReportRequest) (*reportseps.ReportMeta, error) {
	resp := c.client.CreatePlanReport(req)
	if err := ClassifyReportResponse(resp); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, wrapExporter(fmt.Errorf(string(constants.ErrCreateReportFailed), err))
	}
	var meta reportseps.ReportMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, wrapExporter(fmt.Errorf(string(constants.ErrCreateReportFailed), err))
	}
	return &meta, nil
}

func (c *ReportClient) PutLedger(planID string, ledger json.RawMessage) error {
	return ClassifyReportResponse(c.client.PutLedger(planID, ledger))
}

func (c *ReportClient) GetLedger(planID string) (json.RawMessage, error) {
	ledger, err := c.client.GetLedger(planID)
	if errors.Is(err, shared.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, wrapExporter(err)
	}
	return ledger, nil
}
