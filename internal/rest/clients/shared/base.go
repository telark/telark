package shared

import (
	"github.com/plsyro/rest-pkg/base"
	response "github.com/plsyro/rest-pkg/response"
)

type BaseClient struct {
	client *Client
}

func NewBaseClient(service base.Service) *BaseClient {
	return &BaseClient{
		client: New(service),
	}
}

func (c *BaseClient) Create(endpoint base.Endpoint, resource any) *response.GenericResponse {
	return c.client.Create(endpoint, resource)
}

func (c *BaseClient) Update(endpoint base.Endpoint, name string, body map[string]interface{}) *response.GenericResponse {
	return c.client.Update(endpoint, name, body)
}

func (c *BaseClient) Delete(endpoint base.Endpoint, name string) *response.GenericResponse {
	return c.client.Delete(endpoint, name)
}

func (c *BaseClient) Get(endpoint base.Endpoint, name string) (*response.GenericResponse, error) {
	return c.client.Get(endpoint, name)
}

func (c *BaseClient) GetList(endpoint base.Endpoint) ([]any, error) {
	return c.client.GetList(endpoint)
}

func GetTypedBase[T any](client *BaseClient, endpoint base.Endpoint, name string) (*T, error) {
	return GetTyped[T](client.client, endpoint, name)
}

func GetListTypedBase[T any](client *BaseClient, endpoint base.Endpoint) ([]T, error) {
	return GetListTyped[T](client.client, endpoint)
}

func (c *BaseClient) Post(endpoint base.Endpoint) (*response.GenericResponse, error) {
	return c.client.Post(endpoint)
}

func (c *BaseClient) DeleteNoParams(endpoint base.Endpoint) *response.GenericResponse {
	return c.client.DeleteNoParams(endpoint)
}

func (c *BaseClient) FormatEndpoint(template, name string) string {
	return c.client.FormatEndpoint(template, name)
}
