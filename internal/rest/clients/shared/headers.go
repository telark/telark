package shared

import (
	"fmt"

	"github.com/telark/data/errors"
	"github.com/telark/rest/base"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func executeHTTPRequestWithHeaders(
	client *Client,
	method base.Method,
	endpoint base.Endpoint,
	payload []byte,
	headers map[string]string,
) (*base.HTTPResult, error) {
	return doHTTPRequest(client, method, endpoint, payload, headers)
}

func ExecuteRequestWithHeaders(
	client *Client,
	method base.Method,
	endpoint base.Endpoint,
	payload any,
	headers map[string]string,
) *response.GenericResponse {
	var jsonPayload []byte
	var err error
	if payload != nil {
		jsonPayload, err = marshalToJSON(payload)
		if err != nil {
			return CreateErrorResponse(string(errors.ErrRestMarshalPayload), err)
		}
	}

	result, err := executeHTTPRequestWithHeaders(client, method, endpoint, jsonPayload, headers)
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
	result, err := executeHTTPRequestWithHeaders(client, base.Get, endpoint, nil, headers)
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
	result, err := executeHTTPRequestWithHeaders(client, base.Get, endpoint, nil, headers)
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
	result, err := executeHTTPRequestWithHeaders(client, base.Get, endpoint, nil, headers)
	if err != nil {
		return nil, err
	}

	return parseRawJSONResponse[T](result)
}
