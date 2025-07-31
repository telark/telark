package workloads

import (
	appWorkload "github.com/plsyro/data-pkg/resources/workload/app"
	batchWorkload "github.com/plsyro/data-pkg/resources/workload/batch"
	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/clients/shared"
	"github.com/plsyro/rest-pkg/constants"
	workloadsEndpoints "github.com/plsyro/rest-pkg/endpoints/resources/workloads"
	response "github.com/plsyro/rest-pkg/response"
)

type Client struct {
	*shared.BaseResourceClient
}

func NewClient() *Client {
	return &Client{
		BaseResourceClient: shared.NewBaseResourceClient(base.EXPORTER, constants.RESOURCE_TYPE_APP_WORKLOAD),
	}
}

func (c *Client) CreateAppWorkload(workload *appWorkload.AppWorkloadAsResource) *response.GenericResponse {
	return c.CreateResource(workloadsEndpoints.CREATE_APP_WORKLOAD, workload)
}

func (c *Client) CreateBatchWorkload(workload batchWorkload.BatchWorkloadAsResource) *response.GenericResponse {
	return c.CreateResource(workloadsEndpoints.CREATE_BATCH_WORKLOAD, workload)
}

func (c *Client) GetAppWorkloadByName(name string) (*appWorkload.AppWorkloadAsResource, error) {
	return shared.GetResourceByNameTyped[appWorkload.AppWorkloadAsResource](c.BaseResourceClient, workloadsEndpoints.GET_APP_WORKLOAD, name)
}

func (c *Client) GetAllAppWorkloads() ([]*appWorkload.AppWorkloadAsResource, error) {
	return shared.GetAllResourcesTyped[*appWorkload.AppWorkloadAsResource](c.BaseResourceClient, workloadsEndpoints.GET_ALL_APPS_WORKLOADS)
}

func (c *Client) PatchAppWorkload(name string, body map[string]interface{}) *response.GenericResponse {
	return c.PatchResource(workloadsEndpoints.PATCH_APP_WORKLOAD, name, body)
}

func (c *Client) DeleteAppWorkload(name string) *response.GenericResponse {
	return c.DeleteResource(workloadsEndpoints.DELETE_APP_WORKLOAD, name)
}
