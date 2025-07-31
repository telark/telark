package shared

import (
	"net/http"

	"github.com/plsyro/rest-pkg/base"
)

type Client struct {
	httpClient *http.Client
	service    base.Service
}

type BaseClient struct {
	client *Client
}
