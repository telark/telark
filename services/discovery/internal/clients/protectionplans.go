package clients

import (
	"errors"
	"fmt"
	"net/http"
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
	return c.client.Create(userID, req)
}

func (c *ProtectionPlanClient) Get(id string) (*plans.ProtectionPlan, error) {
	return c.client.Get(id)
}

func (c *ProtectionPlanClient) List() ([]plans.ProtectionPlan, error) {
	return c.client.List()
}

func (c *ProtectionPlanClient) Patch(
	userID, id string,
	req planseps.PatchProtectionPlanRequest,
) *response.GenericResponse {
	return c.client.Patch(userID, id, req)
}

func (c *ProtectionPlanClient) Delete(id string) *response.GenericResponse {
	return c.client.Delete(id)
}

func (c *ProtectionPlanClient) PatchOrError(userID, id string, req planseps.PatchProtectionPlanRequest) error {
	resp := c.client.Patch(userID, id, req)
	return ensureOK(resp)
}

func (c *ProtectionPlanClient) PatchRawOrError(userID, id string, body map[string]any) error {
	resp := c.client.PatchRaw(userID, id, body)
	return ensureOK(resp)
}

func (c *ProtectionPlanClient) CreateOrError(userID string, req planseps.CreateProtectionPlanRequest) error {
	resp := c.client.Create(userID, req)
	return ensureOK(resp)
}

func (c *ProtectionPlanClient) DeleteOrError(id string) error {
	resp := c.client.Delete(id)
	return ensureOK(resp)
}

func ensureOK(resp *response.GenericResponse) error {
	if resp == nil {
		return errors.New(string(constants.ErrPatchApplicationReturnedNilResponse))
	}
	if resp.Status != http.StatusOK {
		return fmt.Errorf(string(constants.ErrPatchApplicationFailed), resp.Status, resp.Message)
	}
	return nil
}
