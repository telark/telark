package workloads

import (
	appworkload "github.com/plsyro/data/resources/workloads/app"
	batchworkload "github.com/plsyro/data/resources/workloads/batch"
	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/clients/shared"
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

func (c *Client) Createappworkload(
	workload *appworkload.AppWorkloadAsResource,
) *response.GenericResponse {
	return c.Create(eps.Createappworkload, workload)
}

func (c *Client) CreateBatchWorkload(
	workload batchworkload.BatchWorkloadAsResource,
) *response.GenericResponse {
	return c.Create(eps.CreateBatchWorkload, workload)
}

func (c *Client) GetappworkloadByName(name string) (*appworkload.AppWorkloadAsResource, error) {
	return shared.GetTyped[appworkload.AppWorkloadAsResource](c.Client, eps.Getappworkload,
		name)
}

func (c *Client) GetBatchWorkloadByName(
	name string,
) (*batchworkload.BatchWorkloadAsResource, error) {
	return shared.GetTyped[batchworkload.BatchWorkloadAsResource](
		c.Client, eps.GetBatchWorkload,
		name,
	)
}

func (c *Client) GetAllappworkloads() ([]*appworkload.AppWorkloadAsResource, error) {
	return shared.GetListTyped[*appworkload.AppWorkloadAsResource](c.Client,
		eps.GetAllAppsWorkloads)
}

func (c *Client) GetAllBatchWorkloads() ([]*batchworkload.BatchWorkloadAsResource, error) {
	return shared.GetListTyped[*batchworkload.BatchWorkloadAsResource](c.Client,
		eps.GetAllBatchesWorkloads)
}

func (c *Client) Patchappworkload(name string, body map[string]any) *response.GenericResponse {
	return c.Update(eps.Patchappworkload, name, body)
}

func (c *Client) PatchBatchWorkload(name string, body map[string]any) *response.GenericResponse {
	return c.Update(eps.PatchBatchWorkload, name, body)
}

func (c *Client) Deleteappworkload(name string) *response.GenericResponse {
	return c.Delete(eps.Deleteappworkload, name)
}

func (c *Client) DeleteBatchWorkload(name string) *response.GenericResponse {
	return c.Delete(eps.DeleteBatchWorkload, name)
}
