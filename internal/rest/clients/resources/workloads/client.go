package workloads

import (
	appworkload "github.com/plsyro/data/resources/workloads/app"
	batchworkload "github.com/plsyro/data/resources/workloads/batch"
	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/clients/shared"
	"github.com/plsyro/rest/constants"
	eps "github.com/plsyro/rest/endpoints/resources/workloads"
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

func (c *Client) CreateAppWorkload(
	workload *appworkload.AppWorkloadAsResource,
) *response.GenericResponse {
	return c.Create(eps.CreateAppWorkload, workload)
}

func (c *Client) CreateBatchWorkload(
	workload batchworkload.BatchWorkloadAsResource,
) *response.GenericResponse {
	return c.Create(eps.CreateBatchWorkload, workload)
}

func (c *Client) GetAppWorkloadByName(name string) (*appworkload.AppWorkloadAsResource, error) {
	ep := base.Endpoint(shared.SubstituteEndpointWithParam(
		string(eps.GetAppWorkload),
		constants.NameParam,
		name,
	))
	return shared.GetTyped[appworkload.AppWorkloadAsResource](c.Client, ep)
}

func (c *Client) GetBatchWorkloadByName(
	name string,
) (*batchworkload.BatchWorkloadAsResource, error) {
	ep := base.Endpoint(shared.SubstituteEndpointWithParam(
		string(eps.GetBatchWorkload),
		constants.NameParam,
		name,
	))
	return shared.GetTyped[batchworkload.BatchWorkloadAsResource](c.Client, ep)
}

func (c *Client) GetAllAppWorkloads() ([]*appworkload.AppWorkloadAsResource, error) {
	return shared.GetListTyped[*appworkload.AppWorkloadAsResource](c.Client,
		eps.GetAllAppsWorkloads)
}

func (c *Client) GetAllBatchWorkloads() ([]*batchworkload.BatchWorkloadAsResource, error) {
	return shared.GetListTyped[*batchworkload.BatchWorkloadAsResource](c.Client,
		eps.GetAllBatchesWorkloads)
}

func (c *Client) PatchAppWorkload(name string, body map[string]any) *response.GenericResponse {
	ep := base.Endpoint(shared.SubstituteEndpointWithParam(
		string(eps.PatchAppWorkload),
		constants.NameParam,
		name,
	))
	return c.Update(ep, body)
}

func (c *Client) PatchBatchWorkload(name string, body map[string]any) *response.GenericResponse {
	ep := base.Endpoint(shared.SubstituteEndpointWithParam(
		string(eps.PatchBatchWorkload),
		constants.NameParam,
		name,
	))
	return c.Update(ep, body)
}

func (c *Client) DeleteAppWorkload(name string) *response.GenericResponse {
	ep := base.Endpoint(shared.SubstituteEndpointWithParam(
		string(eps.DeleteAppWorkload),
		constants.NameParam,
		name,
	))
	return c.Delete(ep)
}

func (c *Client) DeleteBatchWorkload(name string) *response.GenericResponse {
	ep := base.Endpoint(shared.SubstituteEndpointWithParam(
		string(eps.DeleteBatchWorkload),
		constants.NameParam,
		name,
	))
	return c.Delete(ep)
}
