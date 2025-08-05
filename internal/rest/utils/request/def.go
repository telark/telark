package request

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	globalerrors "github.com/plsyro/data/errors"
	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/constants"
)

func ParseRequestBody(r *http.Request) (map[string]any, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, base.MaxRequestBodySize))
	if err != nil {
		return nil, fmt.Errorf(string(globalerrors.ErrRestParseRequestBody), err)
	}

	if len(body) == constants.EmptySliceLength {
		return nil, errors.New("empty request body")
	}

	var spec map[string]any
	if err := json.Unmarshal(body, &spec); err != nil {
		return nil, fmt.Errorf(string(globalerrors.ErrRestUnmarshalRequestBodyToJSON), err)
	}
	return spec, nil
}

func CreateGenericRequest(
	method base.Method,
	service base.Service,
	apiVersion base.Version,
	endpoint base.Endpoint,
) base.API {
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

func CreateGenericRequestWithPayload(
	service base.Service,
	apiVersion base.Version,
	endpoint base.Endpoint,
	payload []byte,
) base.API {
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
