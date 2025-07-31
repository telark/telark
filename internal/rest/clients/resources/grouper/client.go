package grouper

import (
	"github.com/plsyro/data-pkg/resources/grouper"
	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/clients/shared"
	"github.com/plsyro/rest-pkg/constants"
	grouperEndpoints "github.com/plsyro/rest-pkg/endpoints/resources/groupers"
	response "github.com/plsyro/rest-pkg/response"
)

type Client struct {
	*shared.BaseResourceClient
}

func NewClient() *Client {
	return &Client{
		BaseResourceClient: shared.NewBaseResourceClient(base.EXPORTER, constants.RESOURCE_TYPE_GROUPER),
	}
}

func (c *Client) CreateGrouper(grouperObj *grouper.GrouperAsResource) *response.GenericResponse {
	return c.CreateResource(grouperEndpoints.CREATE_GROUPER, grouperObj)
}

func (c *Client) GetGrouperByName(name string) (*grouper.GrouperAsResource, error) {
	return shared.GetResourceByNameTyped[grouper.GrouperAsResource](c.BaseResourceClient, grouperEndpoints.GET_GROUPER, name)
}

func (c *Client) GetAllGroupers() ([]*grouper.GrouperAsResource, error) {
	return shared.GetAllResourcesTyped[*grouper.GrouperAsResource](c.BaseResourceClient, grouperEndpoints.GET_ALL_GROUPERS)
}

func (c *Client) PatchGrouper(name string, body map[string]interface{}) *response.GenericResponse {
	return c.PatchResource(grouperEndpoints.PATCH_GROUPER, name, body)
}

func (c *Client) DeleteGrouper(name string) *response.GenericResponse {
	return c.DeleteResource(grouperEndpoints.DELETE_GROUPER, name)
}
