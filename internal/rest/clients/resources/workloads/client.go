package workloads

import (
	appWorkload "github.com/plsyro/data/resources/workloads/app"
	batchWorkload "github.com/plsyro/data/resources/workloads/batch"
	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/clients/shared"
	workloadsEndpoints "github.com/plsyro/rest/endpoints/resources/workloads"
	response "github.com/plsyro/rest/response"
)

type Client struct {
	*shared.Client
}

func NewClient() *Client {
	return &Client{
		Client: shared.New(base.Exporter),
	}
}

func (c *Client) CreateAppWorkload(workload *appWorkload.AppWorkloadAsResource) *response.GenericResponse {
	return c.Create(workloadsEndpoints.CreateAppWorkload, workload)
}

func (c *Client) CreateBatchWorkload(workload batchWorkload.BatchWorkloadAsResource) *response.GenericResponse {
	return c.Create(workloadsEndpoints.CreateBatchWorkload, workload)
}

func (c *Client) GetAppWorkloadByName(name string) (*appWorkload.AppWorkloadAsResource, error) {
	return shared.GetTyped[appWorkload.AppWorkloadAsResource](c.Client, workloadsEndpoints.GetAppWorkload, name)
}

func (c *Client) GetBatchWorkloadByName(name string) (*batchWorkload.BatchWorkloadAsResource, error) {
	return shared.GetTyped[batchWorkload.BatchWorkloadAsResource](c.Client, workloadsEndpoints.GetBatchWorkload, name)
}

func (c *Client) GetAllAppWorkloads() ([]*appWorkload.AppWorkloadAsResource, error) {
	return shared.GetListTyped[*appWorkload.AppWorkloadAsResource](c.Client, workloadsEndpoints.GetAllAppsWorkloads)
}

func (c *Client) GetAllBatchWorkloads() ([]*batchWorkload.BatchWorkloadAsResource, error) {
	return shared.GetListTyped[*batchWorkload.BatchWorkloadAsResource](c.Client, workloadsEndpoints.GetAllBatchesWorkloads)
}

func (c *Client) PatchAppWorkload(name string, body map[string]interface{}) *response.GenericResponse {
	return c.Update(workloadsEndpoints.PatchAppWorkload, name, body)
}

func (c *Client) PatchBatchWorkload(name string, body map[string]interface{}) *response.GenericResponse {
	return c.Update(workloadsEndpoints.PatchBatchWorkload, name, body)
}

func (c *Client) DeleteAppWorkload(name string) *response.GenericResponse {
	return c.Delete(workloadsEndpoints.DeleteAppWorkload, name)
}

func (c *Client) DeleteBatchWorkload(name string) *response.GenericResponse {
	return c.Delete(workloadsEndpoints.DeleteBatchWorkload, name)
}
