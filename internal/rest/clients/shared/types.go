package shared

import (
	"net/http"
	"time"

	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/constants"
)

type Client struct {
	httpClient *http.Client
	service    base.Service
	config     *ClientConfig
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
