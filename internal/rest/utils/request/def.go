package request

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/plsyro/rest-pkg/base"
)

var maxRequestBodySize int64 = 1 << 20 // 1 MB default

// ParseRequestBody reads and parses the request body as JSON into a map.
// Returns an error if reading or unmarshalling fails, or if the body is empty when required.
func ParseRequestBody(r *http.Request, action string, checkEmptyBody bool) (map[string]interface{}, error) {
	if action == "get" || action == "list" {
		return nil, nil
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBodySize))
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	if checkEmptyBody && len(body) == 0 {
		return nil, errors.New("request body is empty")
	}

	var spec map[string]interface{}
	if err := json.Unmarshal(body, &spec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	return spec, nil
}

// CreateGenericRequest creates a base.API struct for a generic request.
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

// CreateGenericRequestWithPayload creates a base.API struct with a payload for a generic request.
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

// BuildSpecPatchBody wraps the given data in a map under the "spec" key.
func BuildSpecPatchBody(data map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"spec": data,
	}
}
