package grouper

import (
	"fmt"
	"net/http"

	"github.com/plsyro/data-pkg/resources/grouper"
	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/clients/shared"
	"github.com/plsyro/rest-pkg/constants"
	grouperEndpoints "github.com/plsyro/rest-pkg/endpoints/resources/groupers"
	response "github.com/plsyro/rest-pkg/response"
	requestUtils "github.com/plsyro/rest-pkg/utils/request"
)

type Client struct {
	sharedClient *shared.Client
}

func NewClient() *Client {
	return &Client{
		sharedClient: shared.NewClient(),
	}
}

func (c *Client) CreateGrouper(grouperObj *grouper.GrouperAsResource) *response.GenericResponse {
	return c.sharedClient.CreateResource(
		base.EXPORTER,
		base.V1,
		grouperEndpoints.CREATE_GROUPER,
		grouperObj,
		constants.RESOURCE_TYPE_GROUPER,
	)
}

func (c *Client) GetGrouperByName(name string) (*grouper.GrouperAsResource, error) {
	apiEndpoint := c.sharedClient.FormatEndpoint(string(grouperEndpoints.GET_GROUPER), name)
	request := requestUtils.CreateGenericRequest(base.GET, base.EXPORTER, base.V1, base.Endpoint(apiEndpoint))
	requestURL, err := request.GenerateURL()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GENERATE_REQUEST_URL_FOR), constants.RESOURCE_TYPE_GROUPER, name, err)
	}

	req, err := http.NewRequest(constants.HTTP_METHOD_GET, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_CREATE_REQUEST), constants.RESOURCE_TYPE_GROUPER, name, err)
	}

	grouperObj, err := shared.DoRequest[grouper.GrouperAsResource](req)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GET_RESOURCE), constants.RESOURCE_TYPE_GROUPER, name, err)
	}
	return grouperObj, nil
}

func (c *Client) GetAllGroupers() ([]*grouper.GrouperAsResource, error) {
	request := requestUtils.CreateGenericRequest(base.GET, base.EXPORTER, base.V1, grouperEndpoints.GET_ALL_GROUPERS)
	requestURL, err := request.GenerateURL()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GENERATE_REQUEST_URL), err)
	}

	req, err := http.NewRequest(constants.HTTP_METHOD_GET, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_CREATE_REQUEST_GENERIC), err)
	}

	groupers, err := shared.DoRequestList[*grouper.GrouperAsResource](req, constants.RESPONSE_ITEMS_KEY)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GET_RESOURCES), constants.RESOURCE_TYPE_GROUPER, err)
	}
	return groupers, nil
}

func (c *Client) PatchGrouper(name string, body map[string]interface{}) *response.GenericResponse {
	return c.sharedClient.PatchResource(
		grouperEndpoints.PATCH_GROUPER,
		name,
		body,
	)
}

func (c *Client) DeleteGrouper(name string) *response.GenericResponse {
	return c.sharedClient.DeleteResource(
		grouperEndpoints.DELETE_GROUPER,
		name,
	)
}
