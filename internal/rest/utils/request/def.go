package request

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/plsyro/rest-pkg/base"
)

func ParseRequestBody(r *http.Request, action string, checkEmptyBody bool) (map[string]interface{}, error) {
	// Skip body parsing for "get" and "list" actions
	if action == "get" || action == "list" {
		return nil, nil
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB limit
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	// Optional empty body check
	if checkEmptyBody && len(body) == 0 {
		return nil, errors.New("request body is empty")
	}

	var spec map[string]interface{}
	if err := json.Unmarshal(body, &spec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	return spec, nil
}

func CreateGetRequest(service base.Service, apiVersion base.Version, endpoint base.Endpoint) base.API {
	return base.API{
		Host: base.Host{
			Schema:  base.HTTP,
			Service: service,
			Port:    base.DEFAULT,
		},
		Version:  apiVersion,
		Endpoint: endpoint,
		Method:   base.GET,
	}
}

func CreatePostRequest(service base.Service, apiVersion base.Version, endpoint base.Endpoint, payload []byte) base.API {
	return base.API{
		Host: base.Host{
			Schema:  base.HTTP,
			Service: service,
			Port:    base.DEFAULT,
		},
		Version:     apiVersion,
		Endpoint:    endpoint,
		ContentType: base.JSON,
		Payload:     payload,
		Method:      base.POST,
	}
}

func CreatePatchRequest(service base.Service, apiVersion base.Version, endpoint base.Endpoint, payload []byte) base.API {
	return base.API{
		Host: base.Host{
			Schema:  base.HTTP,
			Service: service,
			Port:    base.DEFAULT,
		},
		Version:     apiVersion,
		Endpoint:    endpoint,
		ContentType: base.JSON,
		Payload:     payload,
		Method:      base.PATCH,
	}
}
