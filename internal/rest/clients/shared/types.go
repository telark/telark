package shared

import (
	"net/http"
	"time"

	"github.com/plsyro/rest/base"
)

type Client struct {
	httpClient *http.Client
	service    base.Service
	config     *ClientConfig
}

type ClientConfig struct {
	Timeout time.Duration
}

func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		Timeout: 30 * time.Second,
	}
}
