package protection

import (
	"github.com/plsyro/data/plans"
	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/clients/shared"
	"github.com/plsyro/rest/constants"
	eps "github.com/plsyro/rest/endpoints/plans"
	restmapper "github.com/plsyro/rest/mappers"
	"github.com/plsyro/rest/response"
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
	ep := shared.SubstituteEndpointWithParam(string(eps.GetProtectionPlanByID), constants.IDParam, id)
	return shared.GetTyped[plans.ProtectionPlan](c.Client, ep)
}

func (c *Client) List() ([]plans.ProtectionPlan, error) {
	return shared.GetListTyped[plans.ProtectionPlan](c.Client, eps.ListProtectionPlans)
}

func (c *Client) Patch(userID, id string, req eps.PatchProtectionPlanRequest) *response.GenericResponse {
	ep := shared.SubstituteEndpointWithParam(string(eps.PatchProtectionPlanByID), constants.IDParam, id)
	mappedPayload, err := restmapper.MapToJSONPayload(req)
	if err != nil {
		return shared.CreateErrorResponse(string(constants.ErrFailedToCreateHTTPRequest), err)
	}
	return shared.ExecuteRequestWithHeaders(c.Client, base.Patch, ep, mappedPayload, headers(userID))
}

func (c *Client) Delete(id string) *response.GenericResponse {
	ep := shared.SubstituteEndpointWithParam(string(eps.DeleteProtectionPlanByID), constants.IDParam, id)
	return c.Client.Delete(ep)
}

func headers(userID string) map[string]string {
	return map[string]string{HeaderUserID: userID}
}
