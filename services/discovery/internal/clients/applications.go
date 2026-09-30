package clients

import (
	"time"

	appresource "github.com/telark/telark/internal/data/resources/application"
	applicationsclient "github.com/telark/telark/internal/rest/clients/applications"
	"github.com/telark/telark/internal/rest/clients/shared"
	"github.com/telark/telark/internal/rest/response"
	"github.com/telark/telark/services/discovery/internal/constants"
)

const (
	storedApplicationTimeout = 3 * time.Second
	// A full list at 1000 apps is several MB behind a cache rebuild; the
	// single-GET budget above aborted every auto-cleanup cycle at that size.
	listApplicationsTimeout = 30 * time.Second
)

type ExporterClient struct {
	client     *applicationsclient.Client
	listClient *applicationsclient.Client
}

func NewExporterClient() *ExporterClient {
	return &ExporterClient{
		client:     applicationsclient.NewClientWithConfig(&shared.ClientConfig{Timeout: storedApplicationTimeout}),
		listClient: applicationsclient.NewClientWithConfig(&shared.ClientConfig{Timeout: listApplicationsTimeout}),
	}
}

func (c *ExporterClient) GetApplicationByName(name string) (*appresource.Application, error) {
	return guardedExporterGet(func() (*appresource.Application, error) {
		return c.client.GetApplicationByName(name)
	})
}

func (c *ExporterClient) GetApplicationByNameFresh(name string) (*appresource.Application, error) {
	return guardedExporterGet(func() (*appresource.Application, error) {
		return c.client.GetApplicationByNameFresh(name)
	})
}

func (c *ExporterClient) GetAllApplications() ([]*appresource.Application, error) {
	return guardedExporterGet(c.listClient.GetAllApplications)
}

func (c *ExporterClient) PatchApplicationByName(name string, body map[string]any) *response.GenericResponse {
	return guardedResponse(func() *response.GenericResponse {
		return c.client.PatchApplicationByName(name, body)
	})
}

func (c *ExporterClient) DeleteApplicationByName(name string) *response.GenericResponse {
	return guardedResponse(func() *response.GenericResponse {
		return c.client.DeleteApplicationByName(name)
	})
}

func (c *ExporterClient) PatchApplicationByNameOrError(name string, body map[string]any) error {
	return guardedStatusError(constants.ErrPatchApplicationFailed, func() *response.GenericResponse {
		return c.client.PatchApplicationByName(name, body)
	})
}
