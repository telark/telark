package request

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"

	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/rest-pkg/base"
)

func ParseRequestBody(r *http.Request, action string, checkEmptyBody bool) (map[string]any, error) {
	if action == "get" || action == "list" {
		return nil, nil
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, base.MaxRequestBodySize))
	if err != nil {
		return nil, fmt.Errorf(string(errors.ErrRestParseRequestBody), err)
	}

	if checkEmptyBody && len(body) == 0 {
		return nil, stderrors.New("empty request body")
	}

	var spec map[string]any
	if err := json.Unmarshal(body, &spec); err != nil {
		return nil, fmt.Errorf(string(errors.ErrRestUnmarshalRequestBodyToJSON), err)
	}
	return spec, nil
}

func CreateGenericRequest(method base.Method, service base.Service, apiVersion base.Version, endpoint base.Endpoint) base.API {
	return base.API{
		Host: base.Host{
			Schema:  base.HTTP,
			Service: service,
			Port:    base.Default,
		},
		Version:  apiVersion,
		Endpoint: endpoint,
		Method:   method,
	}
}

func CreateGenericRequestWithPayload(_ base.Method, service base.Service, apiVersion base.Version, endpoint base.Endpoint, payload []byte) base.API {
	return base.API{
		Host: base.Host{
			Schema:  base.HTTP,
			Service: service,
			Port:    base.Default,
		},
		Version:     apiVersion,
		Endpoint:    endpoint,
		ContentType: base.JSON,
		Payload:     payload,
		Method:      base.Post,
	}
}

func BuildSpecPatchBody(data map[string]any) map[string]any {
	return map[string]any{
		"spec": data,
	}
}
