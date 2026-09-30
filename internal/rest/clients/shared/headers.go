package shared

import (
	"fmt"

	"github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/rest/base"
	"github.com/telark/telark/internal/rest/constants"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
)

func ExecuteRequestWithHeaders(
	client *Client,
	method base.Method,
	endpoint base.Endpoint,
	payload any,
	headers map[string]string,
) *response.GenericResponse {
	jsonPayload, err := marshalToJSON(payload)
	if err != nil {
		return CreateErrorResponse(string(errors.ErrRestMarshalPayload), err)
	}

	result, err := doHTTPRequest(client, method, endpoint, jsonPayload, headers)
	if err != nil {
		msg := fmt.Sprintf(string(errors.ErrCreateRes), "", err)
		return errorResponse(client.service, msg, err)
	}

	return responseutils.ReadAndParseGenericResponse(result)
}

func GetWithHeaders[T any](
	client *Client,
	endpoint base.Endpoint,
	headers map[string]string,
) (*T, error) {
	result, err := doHTTPRequest(client, base.Get, endpoint, nil, headers)
	if err != nil {
		return nil, err
	}

	return parseSingleResponse[T](result)
}

func GetListWithHeaders[T any](
	client *Client,
	endpoint base.Endpoint,
	headers map[string]string,
) ([]T, error) {
	result, err := doHTTPRequest(client, base.Get, endpoint, nil, headers)
	if err != nil {
		return nil, err
	}

	return parseListResponse[T](result)
}

func GetRawJSONWithHeaders[T any](
	client *Client,
	endpoint base.Endpoint,
	headers map[string]string,
) (*T, error) {
	result, err := doHTTPRequest(client, base.Get, endpoint, nil, headers)
	if err != nil {
		return nil, err
	}

	return parseRawJSONResponse[T](result)
}

// Grant sources: the exporter's response cache sits in Redis, which is untrusted,
// so a record that feeds an authorization decision is always read from the API server.
func GetTypedNoCache[T any](client *Client, endpoint base.Endpoint) (*T, error) {
	return GetWithHeaders[T](client, endpoint, map[string]string{
		constants.HeaderCacheControl: constants.CacheControlNoCache,
	})
}
