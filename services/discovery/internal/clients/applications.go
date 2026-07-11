package clients

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	appresource "github.com/telark/data/resources/application"
	"github.com/telark/discovery/constants"
	applicationsclient "github.com/telark/rest/clients/resources/applications"
	"github.com/telark/rest/clients/shared"
	"github.com/telark/rest/response"
)

const storedApplicationTimeout = 3 * time.Second

type ExporterClient struct {
	client *applicationsclient.Client
}

func NewExporterClient() *ExporterClient {
	cfg := &shared.ClientConfig{Timeout: storedApplicationTimeout}
	return &ExporterClient{
		client: applicationsclient.NewClientWithConfig(cfg),
	}
}

func (c *ExporterClient) GetApplicationByName(name string) (*appresource.Application, error) {
	return c.client.GetApplicationByName(name)
}

func (c *ExporterClient) GetAllApplications() ([]*appresource.Application, error) {
	return c.client.GetAllApplications()
}

func (c *ExporterClient) PatchApplicationByName(name string, body map[string]any) *response.GenericResponse {
	return c.client.PatchApplicationByName(name, body)
}

func (c *ExporterClient) DeleteApplicationByName(name string) *response.GenericResponse {
	return c.client.DeleteApplicationByName(name)
}

func (c *ExporterClient) PatchApplicationByNameOrError(name string, body map[string]any) error {
	resp := c.client.PatchApplicationByName(name, body)
	if resp == nil {
		return errors.New(string(constants.ErrPatchApplicationReturnedNilResponse))
	}
	if resp.Status != http.StatusOK {
		return fmt.Errorf(string(constants.ErrPatchApplicationFailed), resp.Status, resp.Message)
	}
	return nil
}
