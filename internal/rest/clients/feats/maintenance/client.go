package maintenance

import (
	"net/http"

	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/clients/shared"
	"github.com/plsyro/rest-pkg/constants"
	maintenanceEndpoints "github.com/plsyro/rest-pkg/endpoints/feats/maintenance/grouper"
	response "github.com/plsyro/rest-pkg/response"
	requestUtils "github.com/plsyro/rest-pkg/utils/request"
	responseUtils "github.com/plsyro/rest-pkg/utils/response"
)

type Client struct {
	sharedClient *shared.Client
}

func NewClient() *Client {
	return &Client{
		sharedClient: shared.NewClient(),
	}
}

func (c *Client) EnableGrouperMaintenance(body map[string]interface{}) *response.GenericResponse {
	return c.sharedClient.CreateResource(
		base.CONFIGURATOR,
		base.V1,
		maintenanceEndpoints.ENABLE_GROUPER_MAINTENANCE_FEAT,
		body,
		constants.RESOURCE_TYPE_MAINTENANCE,
	)
}

func (c *Client) UpdateGrouperMaintenance(body map[string]interface{}) *response.GenericResponse {
	return c.sharedClient.CreateResource(
		base.CONFIGURATOR,
		base.V1,
		maintenanceEndpoints.UPDATE_GROUPER_MAINTENANCE_FEAT,
		body,
		constants.RESOURCE_TYPE_MAINTENANCE,
	)
}

func (c *Client) RemoveGrouperMaintenance() *response.GenericResponse {
	request := requestUtils.CreateGenericRequest(base.DELETE, base.CONFIGURATOR, base.V1, maintenanceEndpoints.REMOVE_GROUPER_MAINTENANCE_FEAT)
	requestURL, err := request.GenerateURL()
	if err != nil {
		return responseUtils.LogAndReturnResponse(constants.HTTP_STATUS_INTERNAL_SERVER_ERROR, response.OPERATION_ERROR, string(constants.ERROR_FAILED_GENERATE_URL), nil, err)
	}

	req, err := http.NewRequest(constants.HTTP_METHOD_DELETE, requestURL, nil)
	if err != nil {
		return responseUtils.LogAndReturnResponse(constants.HTTP_STATUS_INTERNAL_SERVER_ERROR, response.OPERATION_ERROR, string(constants.ERROR_FAILED_CREATE_DELETE_REQUEST), nil, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return responseUtils.LogAndReturnResponse(constants.HTTP_STATUS_INTERNAL_SERVER_ERROR, response.OPERATION_ERROR, string(constants.ERROR_FAILED_EXECUTE_DELETE_REQUEST), nil, err)
	}

	return responseUtils.ReadAndParseGenericResponse(resp)
}
