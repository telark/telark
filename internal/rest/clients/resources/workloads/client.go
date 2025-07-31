package workloads

import (
	appWorkload "github.com/plsyro/data-pkg/resources/workload/app"
	batchWorkload "github.com/plsyro/data-pkg/resources/workload/batch"
	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/clients/shared"
	workloadsEndpoints "github.com/plsyro/rest-pkg/endpoints/resources/workloads"
	response "github.com/plsyro/rest-pkg/response"
)

type Client struct {
	*shared.BaseClient
}

func NewClient() *Client {
	return &Client{
		BaseClient: shared.NewBaseClient(base.EXPORTER),
	}
}

func (c *Client) CreateAppWorkload(workload *appWorkload.AppWorkloadAsResource) *response.GenericResponse {
	return c.Create(workloadsEndpoints.CREATE_APP_WORKLOAD, workload)
}

func (c *Client) CreateBatchWorkload(workload batchWorkload.BatchWorkloadAsResource) *response.GenericResponse {
	return c.Create(workloadsEndpoints.CREATE_BATCH_WORKLOAD, workload)
}

func (c *Client) GetAppWorkloadByName(name string) (*appWorkload.AppWorkloadAsResource, error) {
	return shared.GetTypedBase[appWorkload.AppWorkloadAsResource](c.BaseClient, workloadsEndpoints.GET_APP_WORKLOAD, name)
}

func (c *Client) GetBatchWorkloadByName(name string) (*batchWorkload.BatchWorkloadAsResource, error) {
	return shared.GetTypedBase[batchWorkload.BatchWorkloadAsResource](c.BaseClient, workloadsEndpoints.GET_BATCH_WORKLOAD, name)
}

func (c *Client) GetAllAppWorkloads() ([]*appWorkload.AppWorkloadAsResource, error) {
	return shared.GetListTypedBase[*appWorkload.AppWorkloadAsResource](c.BaseClient, workloadsEndpoints.GET_ALL_APPS_WORKLOADS)
}

func (c *Client) GetAllBatchWorkloads() ([]*batchWorkload.BatchWorkloadAsResource, error) {
	return shared.GetListTypedBase[*batchWorkload.BatchWorkloadAsResource](c.BaseClient, workloadsEndpoints.GET_ALL_BATCHES_WORKLOADS)
}

func (c *Client) PatchAppWorkload(name string, body map[string]interface{}) *response.GenericResponse {
	return c.Update(workloadsEndpoints.PATCH_APP_WORKLOAD, name, body)
}

func (c *Client) PatchBatchWorkload(name string, body map[string]interface{}) *response.GenericResponse {
	return c.Update(workloadsEndpoints.PATCH_BATCH_WORKLOAD, name, body)
}

func (c *Client) DeleteAppWorkload(name string) *response.GenericResponse {
	return c.Delete(workloadsEndpoints.DELETE_APP_WORKLOAD, name)
}

func (c *Client) DeleteBatchWorkload(name string) *response.GenericResponse {
	return c.Delete(workloadsEndpoints.DELETE_BATCH_WORKLOAD, name)
}
