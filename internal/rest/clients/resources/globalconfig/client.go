package globalconfig

import (
	"fmt"
	"strings"
	"time"

	globalconfigresource "github.com/plsyro/data/resources/globalconfig"
	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/clients/shared"
	"github.com/plsyro/rest/constants"
	eps "github.com/plsyro/rest/endpoints/resources/globalconfig"
	"github.com/plsyro/rest/response"
)

type Client struct {
	*shared.Client
}

func NewClient() *Client {
	return &Client{
		Client: shared.New(base.Exporter),
	}
}

func NewClientWithConfig(cfg *shared.ClientConfig) *Client {
	return &Client{
		Client: shared.NewWithConfig(base.Exporter, cfg),
	}
}

func (c *Client) GetGlobalConfig() (*globalconfigresource.GlobalConfig, error) {
	return shared.GetTyped[globalconfigresource.GlobalConfig](c.Client, eps.GetGlobalConfig)
}

func (c *Client) PatchGlobalConfig(body map[string]any) *response.GenericResponse {
	for attempt := 1; attempt <= constants.GlobalConfigPatchRetryAttempts; attempt++ {
		resourceVersion, err := c.getResourceVersion()
		if err != nil {
			return shared.CreateErrorResponse(fmt.Sprintf(string(constants.ErrGlobalConfigReadFailed), err), err)
		}

		payload := withResourceVersion(body, resourceVersion)
		resp := c.Update(eps.PatchGlobalConfig, payload)
		if !isConflictResponse(resp) {
			return resp
		}

		if attempt < constants.GlobalConfigPatchRetryAttempts {
			time.Sleep(constants.GlobalConfigPatchRetryBackoff)
		}
	}

	return shared.CreateErrorResponse(
		fmt.Sprintf(
			string(constants.ErrGlobalConfigPatchRetryFailure),
			constants.GlobalConfigPatchRetryAttempts,
		),
		nil,
	)
}

func (c *Client) getResourceVersion() (string, error) {
	resp, err := c.Get(eps.GetGlobalConfig)
	if err != nil {
		return constants.EmptyString, err
	}

	data, ok := resp.Data.(map[string]any)
	if !ok || data == nil {
		return constants.EmptyString, fmt.Errorf(string(constants.ErrGlobalConfigResponseData))
	}
	metadata, ok := data[constants.MetadataField].(map[string]any)
	if !ok || metadata == nil {
		return constants.EmptyString, fmt.Errorf(string(constants.ErrGlobalConfigMetadataMissing))
	}
	rawVersion, ok := metadata[constants.ResourceVersionField]
	if !ok {
		return constants.EmptyString, fmt.Errorf(string(constants.ErrGlobalConfigVersionMissing))
	}
	resourceVersion := strings.TrimSpace(fmt.Sprintf("%v", rawVersion))
	if resourceVersion == constants.EmptyString {
		return constants.EmptyString, fmt.Errorf(string(constants.ErrGlobalConfigVersionMissing))
	}
	return resourceVersion, nil
}

func withResourceVersion(body map[string]any, resourceVersion string) map[string]any {
	spec := make(map[string]any, len(body))
	for key, value := range body {
		spec[key] = value
	}

	return map[string]any{
		constants.SpecField: spec,
		constants.MetadataField: map[string]any{
			constants.ResourceVersionField: resourceVersion,
		},
	}
}

func isConflictResponse(resp *response.GenericResponse) bool {
	if resp == nil {
		return false
	}
	if resp.Status == constants.GlobalConfigConflictStatus {
		return true
	}
	msg := strings.ToLower(resp.Message)
	return strings.Contains(msg, constants.GlobalConfigConflictMessageOne) ||
		strings.Contains(msg, constants.GlobalConfigConflictMessageTwo)
}
