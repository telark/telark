package shared

import (
	"net/http"
	"time"

	"github.com/telark/telark/internal/rest/base"
	"github.com/telark/telark/internal/rest/constants"
)

type Client struct {
	httpClient *http.Client
	service    base.Service
	config     *ClientConfig
	params     map[string]string
}

// Path parameter values are often credentials or PII: merged in only when the URL
// is built, so any base.Endpoint reaching a log or error is still a template.
func (c *Client) WithParams(params map[string]string) *Client {
	clone := *c
	clone.params = params
	return &clone
}

type ClientConfig struct {
	Timeout time.Duration
}

type Items[T any] struct {
	Items []T `json:"items"`
}

type listDataResponse[T any] struct {
	Data Items[T] `json:"data"`
}

type singleDataResponse[T any] struct {
	Data T `json:"data"`
}

func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		Timeout: constants.DefaultTimeout * time.Second,
	}
}
