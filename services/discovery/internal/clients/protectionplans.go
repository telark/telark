package clients

import (
	"time"

	"github.com/telark/data/plans"
	protectionclient "github.com/telark/rest/clients/plans/protection"
	"github.com/telark/rest/clients/shared"
	planseps "github.com/telark/rest/endpoints/plans"
	"github.com/telark/rest/response"

	"github.com/telark/discovery/internal/constants"
)

const protectionPlanTimeout = 30 * time.Second

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
	return guardedExporterGet(func() (*plans.ProtectionPlan, error) {
		return c.client.Get(id)
	})
}

func (c *ProtectionPlanClient) List() ([]plans.ProtectionPlan, error) {
	return guardedExporterGet(c.client.List)
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
	return guardedStatusError(constants.ErrPatchApplicationFailed, func() *response.GenericResponse {
		return c.client.Patch(userID, id, req)
	})
}

func (c *ProtectionPlanClient) PatchRawOrError(userID, id string, body map[string]any) error {
	return guardedStatusError(constants.ErrPatchApplicationFailed, func() *response.GenericResponse {
		return c.client.PatchRaw(userID, id, body)
	})
}

func (c *ProtectionPlanClient) CreateOrError(userID string, req planseps.CreateProtectionPlanRequest) error {
	return guardedStatusError(constants.ErrPatchApplicationFailed, func() *response.GenericResponse {
		return c.client.Create(userID, req)
	})
}

func (c *ProtectionPlanClient) DeleteOrError(id string) error {
	return guardedStatusError(constants.ErrPatchApplicationFailed, func() *response.GenericResponse {
		return c.client.Delete(id)
	})
}
