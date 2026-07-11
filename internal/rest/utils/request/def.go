package request

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	globalerrors "github.com/telark/data/errors"
	"github.com/telark/rest/base"
	"github.com/telark/rest/constants"
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

func getServiceConfig(service base.Service) (base.Schema, base.Port) {
	switch service {
	case base.AdmissionOperator:
		return base.HTTPS, base.HTTPSPort
	default:
		return base.HTTP, base.Default
	}
}

func CreateGenericRequest(
	method base.Method,
	service base.Service,
	apiVersion base.Version,
	endpoint base.Endpoint,
) base.API {
	schema, port := getServiceConfig(service)
	return base.API{
		Host: base.Host{
			Schema:  schema,
			Service: service,
			Port:    port,
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
	schema, port := getServiceConfig(service)
	return base.API{
		Host: base.Host{
			Schema:  schema,
			Service: service,
			Port:    port,
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
