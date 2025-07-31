package grouper

import (
	grouperResource "github.com/plsyro/data-pkg/resources/grouper"
	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/clients/shared"
	grouperEndpoints "github.com/plsyro/rest-pkg/endpoints/resources/groupers"
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

func (c *Client) CreateGrouper(grouper *grouperResource.GrouperAsResource) *response.GenericResponse {
	return c.Create(grouperEndpoints.CREATE_GROUPER, grouper)
}

func (c *Client) GetGrouperByName(name string) (*grouperResource.GrouperAsResource, error) {
	return shared.GetTypedBase[grouperResource.GrouperAsResource](c.BaseClient, grouperEndpoints.GET_GROUPER, name)
}

func (c *Client) GetAllGroupers() ([]*grouperResource.GrouperAsResource, error) {
	return shared.GetListTypedBase[*grouperResource.GrouperAsResource](c.BaseClient, grouperEndpoints.GET_ALL_GROUPERS)
}

func (c *Client) PatchGrouper(name string, body map[string]interface{}) *response.GenericResponse {
	return c.Update(grouperEndpoints.PATCH_GROUPER, name, body)
}

func (c *Client) DeleteGrouper(name string) *response.GenericResponse {
	return c.Delete(grouperEndpoints.DELETE_GROUPER, name)
}
