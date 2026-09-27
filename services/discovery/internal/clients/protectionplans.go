package clients

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	dataerrors "github.com/telark/data/errors"
	"github.com/telark/data/plans"
	protectionclient "github.com/telark/rest/clients/plans/protection"
	"github.com/telark/rest/clients/shared"
	planseps "github.com/telark/rest/endpoints/plans"
	"github.com/telark/rest/response"

	"github.com/telark/discovery/internal/constants"
)

const (
	protectionPlanTimeout = 30 * time.Second
	jsonEnvelopeStart     = "{"
)

type ProtectionPlanClient struct {
	client *protectionclient.Client
}

func NewProtectionPlanClient() *ProtectionPlanClient {
	cfg := &shared.ClientConfig{Timeout: protectionPlanTimeout}
	return &ProtectionPlanClient{
		client: protectionclient.NewClientWithConfig(cfg),
	}
}

func (c *ProtectionPlanClient) Create(
	userID string,
	req planseps.CreateProtectionPlanRequest,
) *response.GenericResponse {
	return guardedResponse(func() *response.GenericResponse {
		return c.client.Create(userID, req)
	})
}

func (c *ProtectionPlanClient) Get(id string) (*plans.ProtectionPlan, error) {
	plan, err := guardedExporterGet(func() (*plans.ProtectionPlan, error) {
		return c.client.Get(id)
	})
	return plan, wrapExporter(err)
}

func (c *ProtectionPlanClient) List() ([]plans.ProtectionPlan, error) {
	list, err := guardedExporterGet(c.client.List)
	return list, wrapExporter(err)
}

func (c *ProtectionPlanClient) Patch(
	userID, id string,
	req planseps.PatchProtectionPlanRequest,
) *response.GenericResponse {
	return guardedResponse(func() *response.GenericResponse {
		return c.client.Patch(userID, id, req)
	})
}

func (c *ProtectionPlanClient) Delete(id string) *response.GenericResponse {
	return guardedResponse(func() *response.GenericResponse {
		return c.client.Delete(id)
	})
}

func (c *ProtectionPlanClient) PatchOrError(userID, id string, req planseps.PatchProtectionPlanRequest) error {
	return exporterStatusError(constants.ErrPatchProtectionPlanFailed, func() *response.GenericResponse {
		return c.client.Patch(userID, id, req)
	})
}

func (c *ProtectionPlanClient) PatchRawOrError(userID, id string, body map[string]any) error {
	return exporterStatusError(constants.ErrPatchProtectionPlanFailed, func() *response.GenericResponse {
		return c.client.PatchRaw(userID, id, body)
	})
}

func (c *ProtectionPlanClient) CreateOrError(userID string, req planseps.CreateProtectionPlanRequest) error {
	return exporterStatusError(constants.ErrCreateProtectionPlanFailed, func() *response.GenericResponse {
		return c.client.Create(userID, req)
	})
}

func (c *ProtectionPlanClient) DeleteOrError(id string) error {
	return exporterStatusError(constants.ErrDeleteProtectionPlanFailed, func() *response.GenericResponse {
		return c.client.Delete(id)
	})
}

// ErrPlanNotFound lets handlers map a missing plan to 404 without importing the rest clients.
var ErrPlanNotFound = shared.ErrNotFound

// ExporterError marks every failure that came from the exporter call itself, so the API
// answers 503 instead of a blanket 500.
type ExporterError struct{ err error }

func (e *ExporterError) Error() string { return e.err.Error() }
func (e *ExporterError) Unwrap() error { return e.err }

func IsExporterFailure(err error) bool {
	var target *ExporterError
	return errors.As(err, &target)
}

func wrapExporter(err error) error {
	if err == nil {
		return nil
	}
	return &ExporterError{err: err}
}

func exporterStatusError(failFormat dataerrors.Error, op func() *response.GenericResponse) error {
	return wrapExporter(guardedStatusError(failFormat, unwrapMessage(op)))
}

func unwrapMessage(op func() *response.GenericResponse) func() *response.GenericResponse {
	return func() *response.GenericResponse {
		resp := op()
		if resp != nil {
			resp.Message = innerMessage(resp.Message)
		}
		return resp
	}
}

// The exporter forwards its failure as a JSON envelope inside the message; only the inner
// message is meaningful to an API caller.
func innerMessage(message string) string {
	start := strings.Index(message, jsonEnvelopeStart)
	if start < constants.DefaultInitValue {
		return message
	}
	var envelope struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(message[start:]), &envelope); err != nil ||
		envelope.Message == constants.EmptyString {
		return message
	}
	return envelope.Message
}
