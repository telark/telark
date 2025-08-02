package request

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/plsyro/rest-pkg/base"
)

var maxRequestBodySize int64 = 1 << 20 // 1 MB default

func ParseRequestBody(r *http.Request, action string, checkEmptyBody bool) (map[string]any, error) {
	if action == "get" || action == "list" {
		return nil, nil
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBodySize))
	if err != nil {
		return nil, fmt.Errorf("failed to parse request body: %w", err)
	}

	if checkEmptyBody && len(body) == 0 {
		return nil, fmt.Errorf("request body is empty")
	}

	var spec map[string]any
	if err := json.Unmarshal(body, &spec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request body to JSON: %w", err)
	}
	return spec, nil
}

func CreateGenericRequest(method base.Method, service base.Service, apiVersion base.Version, endpoint base.Endpoint) base.API {
	return base.API{
		Host: base.Host{
			Schema:  base.HTTP,
			Service: service,
			Port:    base.DEFAULT,
		},
		Version:  apiVersion,
		Endpoint: endpoint,
		Method:   method,
	}
}

func CreateGenericRequestWithPayload(method base.Method, service base.Service, apiVersion base.Version, endpoint base.Endpoint, payload []byte) base.API {
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

func BuildSpecPatchBody(data map[string]any) map[string]any {
	return map[string]any{
		"spec": data,
	}
}
