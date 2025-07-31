package workloads

import (
	"fmt"
	"net/http"

	appWorkload "github.com/plsyro/data-pkg/resources/workload/app"
	batchWorkload "github.com/plsyro/data-pkg/resources/workload/batch"
	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/clients/shared"
	"github.com/plsyro/rest-pkg/constants"
	workloadsEndpoints "github.com/plsyro/rest-pkg/endpoints/resources/workloads"
	response "github.com/plsyro/rest-pkg/response"
	requestUtils "github.com/plsyro/rest-pkg/utils/request"
)

// Client provides a unified interface for workload operations
type Client struct {
	sharedClient *shared.Client
}

// NewClient creates a new Client instance
func NewClient() *Client {
	return &Client{
		sharedClient: shared.NewClient(),
	}
}

// CreateAppWorkload creates a new app workload
func (c *Client) CreateAppWorkload(workload *appWorkload.AppWorkloadAsResource) *response.GenericResponse {
	return c.sharedClient.CreateResource(
		base.EXPORTER,
		base.V1,
		workloadsEndpoints.CREATE_APP_WORKLOAD,
		workload,
		constants.RESOURCE_TYPE_APP_WORKLOAD,
	)
}

// CreateBatchWorkload creates a new batch workload
func (c *Client) CreateBatchWorkload(workload batchWorkload.BatchWorkloadAsResource) *response.GenericResponse {
	return c.sharedClient.CreateResource(
		base.EXPORTER,
		base.V1,
		workloadsEndpoints.CREATE_BATCH_WORKLOAD,
		workload,
		constants.RESOURCE_TYPE_BATCH_WORKLOAD,
	)
}

// GetAppWorkloadByName retrieves an app workload by name
func (c *Client) GetAppWorkloadByName(name string) (*appWorkload.AppWorkloadAsResource, error) {
	apiEndpoint := c.sharedClient.FormatEndpoint(string(workloadsEndpoints.GET_APP_WORKLOAD), name)
	request := requestUtils.CreateGenericRequest(base.GET, base.EXPORTER, base.V1, base.Endpoint(apiEndpoint))
	requestURL, err := request.GenerateURL()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GENERATE_REQUEST_URL_FOR), constants.RESOURCE_TYPE_APP_WORKLOAD, name, err)
	}

	req, err := http.NewRequest(constants.HTTP_METHOD_GET, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for app workload %s: %w", name, err)
	}

	appWorkload, err := shared.DoRequest[appWorkload.AppWorkloadAsResource](req)
	if err != nil {
		return nil, fmt.Errorf("failed to get app workload %s: %w", name, err)
	}
	return appWorkload, nil
}

// GetAllAppWorkloads retrieves all app workloads
func (c *Client) GetAllAppWorkloads() ([]*appWorkload.AppWorkloadAsResource, error) {
	request := requestUtils.CreateGenericRequest(base.GET, base.EXPORTER, base.V1, workloadsEndpoints.GET_ALL_APPS_WORKLOADS)
	requestURL, err := request.GenerateURL()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GENERATE_REQUEST_URL), err)
	}

	req, err := http.NewRequest(constants.HTTP_METHOD_GET, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	appWorkloads, err := shared.DoRequestList[*appWorkload.AppWorkloadAsResource](req, constants.RESPONSE_ITEMS_KEY)
	if err != nil {
		return nil, fmt.Errorf("failed to get app workloads: %w", err)
	}
	return appWorkloads, nil
}

// PatchAppWorkload updates an app workload
func (c *Client) PatchAppWorkload(name string, body map[string]interface{}) *response.GenericResponse {
	return c.sharedClient.PatchResource(
		workloadsEndpoints.PATCH_APP_WORKLOAD,
		name,
		body,
	)
}

// DeleteAppWorkload deletes an app workload
func (c *Client) DeleteAppWorkload(name string) *response.GenericResponse {
	return c.sharedClient.DeleteResource(
		workloadsEndpoints.DELETE_APP_WORKLOAD,
		name,
	)
}
