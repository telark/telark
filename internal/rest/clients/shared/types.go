package shared

import (
	"net/http"
	"time"

	"github.com/telark/rest/base"
	"github.com/telark/rest/constants"
)

type Client struct {
	httpClient *http.Client
	service    base.Service
	config     *ClientConfig
	params     map[string]string
}

// Path parameter values are frequently credentials or PII. They are carried
// beside the endpoint and only merged in when the URL is built, so the
// base.Endpoint that reaches a log line or an error can only ever be a template.
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
