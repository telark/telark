package shared

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/telark/data/errors"
	"github.com/telark/rest/base"
	"github.com/telark/rest/constants"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func executeHTTPRequestWithHeaders(
	client *Client,
	method base.Method,
	endpoint base.Endpoint,
	payload []byte,
	headers map[string]string,
) (*http.Response, error) {
	url, err := buildRequestURL(client, method, endpoint, payload)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedToGenerateRequestURL), err)
	}

	req, err := http.NewRequest(string(method), url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedToCreateHTTPRequest), err)
	}

	if payload != nil {
		req.Header.Set("Content-Type", string(base.JSON))
	}

	// Add custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	start := time.Now()
	resp, doErr := client.httpClient.Do(req)
	observeExporterCall(client.service, method, endpoint, resp, doErr, time.Since(start))
	return resp, doErr
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

	//nolint:bodyclose // responseutils.ReadAndParseGenericResponse handles closing
	resp, err := executeHTTPRequestWithHeaders(client, method, endpoint, jsonPayload, headers)
	if err != nil {
		msg := fmt.Sprintf(string(errors.ErrCreateRes), "", err)
		return CreateErrorResponse(msg, err)
	}

	return responseutils.ReadAndParseGenericResponse(resp)
}

func GetWithHeaders[T any](
	client *Client,
	endpoint base.Endpoint,
	headers map[string]string,
) (*T, error) {
	//nolint:bodyclose // defer responseutils.CloseResponseBody handles closing
	resp, err := executeHTTPRequestWithHeaders(client, base.Get, endpoint, nil, headers)
	if err != nil {
		return nil, err
	}
	defer responseutils.CloseResponseBody(resp)

	return parseSingleResponse[T](resp)
}

func GetListWithHeaders[T any](
	client *Client,
	endpoint base.Endpoint,
	headers map[string]string,
) ([]T, error) {
	//nolint:bodyclose // defer responseutils.CloseResponseBody handles closing
	resp, err := executeHTTPRequestWithHeaders(client, base.Get, endpoint, nil, headers)
	if err != nil {
		return nil, err
	}
	defer responseutils.CloseResponseBody(resp)

	return parseListResponse[T](resp)
}

func GetRawJSONWithHeaders[T any](
	client *Client,
	endpoint base.Endpoint,
	headers map[string]string,
) (*T, error) {
	//nolint:bodyclose // defer responseutils.CloseResponseBody handles closing
	resp, err := executeHTTPRequestWithHeaders(client, base.Get, endpoint, nil, headers)
	if err != nil {
		return nil, err
	}
	defer responseutils.CloseResponseBody(resp)

	return parseRawJSONResponse[T](resp)
}
