package config

import (
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/telark/telark/internal/data/resources/telarkconfig"
	"github.com/telark/telark/internal/rest/base"
	"github.com/telark/telark/internal/rest/clients/shared"
	"github.com/telark/telark/internal/rest/constants"
	eps "github.com/telark/telark/internal/rest/endpoints/config"
	"github.com/telark/telark/internal/rest/response"
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

func (c *Client) GetConfig() (*telarkconfig.TelarkConfig, error) {
	return shared.GetTyped[telarkconfig.TelarkConfig](c.Client, eps.GetConfig)
}

func (c *Client) PatchConfig(body map[string]any) *response.GenericResponse {
	for attempt := range constants.ConfigPatchRetryAttempts {
		resourceVersion, err := c.getResourceVersion()
		if err != nil {
			return shared.CreateErrorResponse(fmt.Sprintf(string(constants.ErrConfigReadFailed), err), err)
		}

		payload := withResourceVersion(body, resourceVersion)
		resp := c.Update(eps.PatchConfig, payload)
		if !isConflictResponse(resp) {
			return resp
		}

		if attempt < constants.ConfigPatchRetryAttempts-constants.SecondIndex {
			time.Sleep(constants.ConfigPatchRetryBackoff)
		}
	}

	return shared.CreateErrorResponse(
		fmt.Sprintf(
			string(constants.ErrConfigPatchRetryFailure),
			constants.ConfigPatchRetryAttempts,
		),
		nil,
	)
}

func (c *Client) getResourceVersion() (string, error) {
	resp, err := c.Get(eps.GetConfig)
	if err != nil {
		return constants.EmptyString, err
	}

	data, ok := resp.Data.(map[string]any)
	if !ok || data == nil {
		return constants.EmptyString, errors.New(string(constants.ErrConfigResponseData))
	}
	metadata, ok := data[constants.MetadataField].(map[string]any)
	if !ok || metadata == nil {
		return constants.EmptyString, errors.New(string(constants.ErrConfigMetadataMissing))
	}
	rawVersion, ok := metadata[constants.ResourceVersionField]
	if !ok {
		return constants.EmptyString, errors.New(string(constants.ErrConfigVersionMissing))
	}
	resourceVersion := strings.TrimSpace(fmt.Sprintf("%v", rawVersion))
	if resourceVersion == constants.EmptyString {
		return constants.EmptyString, errors.New(string(constants.ErrConfigVersionMissing))
	}
	return resourceVersion, nil
}

func withResourceVersion(body map[string]any, resourceVersion string) map[string]any {
	spec := make(map[string]any, len(body))
	maps.Copy(spec, body)

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
	if resp.Status == constants.ConfigConflictStatus {
		return true
	}
	msg := strings.ToLower(resp.Message)
	return strings.Contains(msg, constants.ConfigConflictMessageOne) ||
		strings.Contains(msg, constants.ConfigConflictMessageTwo)
}
