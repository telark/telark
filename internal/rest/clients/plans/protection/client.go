package protection

import (
	"github.com/telark/data/plans"
	"github.com/telark/rest/base"
	"github.com/telark/rest/clients/shared"
	"github.com/telark/rest/constants"
	eps "github.com/telark/rest/endpoints/plans"
	restmapper "github.com/telark/rest/mappers"
	"github.com/telark/rest/response"
)

const HeaderUserID = "X-User-ID"

type Client struct {
	*shared.Client
}

func NewClient() *Client {
	return &Client{Client: shared.New(base.Exporter)}
}

func NewClientWithConfig(cfg *shared.ClientConfig) *Client {
	return &Client{Client: shared.NewWithConfig(base.Exporter, cfg)}
}

func (c *Client) Create(userID string, req eps.CreateProtectionPlanRequest) *response.GenericResponse {
	mappedPayload, err := restmapper.MapToJSONPayload(req)
	if err != nil {
		return shared.CreateErrorResponse(string(constants.ErrFailedToCreateHTTPRequest), err)
	}
	return shared.ExecuteRequestWithHeaders(
		c.Client, base.Post, eps.CreateProtectionPlan, mappedPayload, headers(userID),
	)
}

func (c *Client) Get(id string) (*plans.ProtectionPlan, error) {
	byID := c.WithParams(map[string]string{constants.IDParam: id})
	return shared.GetTyped[plans.ProtectionPlan](byID, eps.GetProtectionPlanByID)
}

func (c *Client) List() ([]plans.ProtectionPlan, error) {
	return shared.GetListTyped[plans.ProtectionPlan](c.Client, eps.ListProtectionPlans)
}

func (c *Client) Patch(userID, id string, req eps.PatchProtectionPlanRequest) *response.GenericResponse {
	mappedPayload, err := restmapper.MapToJSONPayload(req)
	if err != nil {
		return shared.CreateErrorResponse(string(constants.ErrFailedToCreateHTTPRequest), err)
	}
	byID := c.WithParams(map[string]string{constants.IDParam: id})
	return shared.ExecuteRequestWithHeaders(
		byID, base.Patch, eps.PatchProtectionPlanByID, mappedPayload, headers(userID),
	)
}

// PatchRaw sends a JSON merge patch using a caller-supplied body. nil map entries serialize as JSON null,
// allowing nullable fields like terminatedAt/terminatedBy/reason to be cleared on the underlying CRD.
func (c *Client) PatchRaw(userID, id string, body map[string]any) *response.GenericResponse {
	byID := c.WithParams(map[string]string{constants.IDParam: id})
	return shared.ExecuteRequestWithHeaders(
		byID, base.Patch, eps.PatchProtectionPlanByID, body, headers(userID),
	)
}

func (c *Client) Delete(id string) *response.GenericResponse {
	byID := c.WithParams(map[string]string{constants.IDParam: id})
	return byID.Delete(eps.DeleteProtectionPlanByID)
}

func headers(userID string) map[string]string {
	return map[string]string{HeaderUserID: userID}
}
