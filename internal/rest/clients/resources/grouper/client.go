package grouper

import (
	grouperResource "github.com/plsyro/data-pkg/resources/grouper"
	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/clients/shared"
	grouperEndpoints "github.com/plsyro/rest-pkg/endpoints/resources/groupers"
	response "github.com/plsyro/rest-pkg/response"
)

type Client struct {
	*shared.Client
}

func NewClient() *Client {
	return &Client{
		Client: shared.New(base.EXPORTER),
	}
}

func (c *Client) CreateGrouper(grouper *grouperResource.GrouperAsResource) *response.GenericResponse {
	return c.Create(grouperEndpoints.CreateGrouper, grouper)
}

func (c *Client) GetGrouperByName(name string) (*grouperResource.GrouperAsResource, error) {
	return shared.GetTyped[grouperResource.GrouperAsResource](c.Client, grouperEndpoints.GetGrouper, name)
}

func (c *Client) GetAllGroupers() ([]*grouperResource.GrouperAsResource, error) {
	return shared.GetListTyped[*grouperResource.GrouperAsResource](c.Client, grouperEndpoints.GetAllGroupers)
}

func (c *Client) PatchGrouper(name string, body map[string]interface{}) *response.GenericResponse {
	return c.Update(grouperEndpoints.PatchGrouper, name, body)
}

func (c *Client) DeleteGrouper(name string) *response.GenericResponse {
	return c.Delete(grouperEndpoints.DeleteGrouper, name)
}
