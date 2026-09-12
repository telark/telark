package shared

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/telark/data/errors"
	globalshared "github.com/telark/data/shared"
	"github.com/telark/rest/base"
	"github.com/telark/rest/connectivity"
	"github.com/telark/rest/constants"
	"github.com/telark/rest/response"
	requestutils "github.com/telark/rest/utils/request"
	responseutils "github.com/telark/rest/utils/response"
)

func resolveEndpoint(endpoint base.Endpoint, params map[string]string) base.Endpoint {
	resolved := string(endpoint)
	for placeholder, value := range params {
		resolved = strings.Replace(resolved, placeholder, value, constants.ReplaceCount)
	}
	return base.Endpoint(resolved)
}

// sanitizeEndpoint yields the form safe to expose: the template keeps the path
// parameters unresolved, and the query string is dropped because callers build
// it themselves and may embed a user id or an email in it.
func sanitizeEndpoint(endpoint base.Endpoint) base.Endpoint {
	path, _, _ := strings.Cut(string(endpoint), constants.QuerySeparator)
	return base.Endpoint(path)
}

// A *url.Error embeds the full request URL, so any error escaping an HTTP call
// carries every resolved path parameter with it unless it is rebuilt here.
func redactEndpointError(err error, endpoint base.Endpoint) error {
	var urlErr *url.Error
	if !stderrors.As(err, &urlErr) {
		return err
	}
	return fmt.Errorf(string(constants.ErrEndpointCall), urlErr.Op, sanitizeEndpoint(endpoint), urlErr.Err)
}

func buildRequestURL(
	client *Client,
	method base.Method,
	endpoint base.Endpoint,
	payload []byte,
) (string, error) {
	resolved := resolveEndpoint(endpoint, client.params)

	var req base.API
	if payload != nil {
		req = requestutils.CreateGenericRequestWithPayload(
			client.service,
			base.V1,
			resolved,
			payload,
		)
	} else {
		req = requestutils.CreateGenericRequest(
			method, client.service, base.V1, resolved)
	}

	return req.GenerateURL()
}

// Sole caller of httpClient.Do, so observation and error redaction cannot be
// bypassed by a new request path.
func doHTTPRequest(
	client *Client,
	method base.Method,
	endpoint base.Endpoint,
	payload []byte,
	headers map[string]string,
) (*http.Response, error) {
	requestURL, err := buildRequestURL(client, method, endpoint, payload)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedToGenerateRequestURL), err)
	}

	req, err := http.NewRequest(string(method), requestURL, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf(
			string(constants.ErrFailedToCreateHTTPRequest), redactEndpointError(err, endpoint),
		)
	}

	if payload != nil {
		req.Header.Set("Content-Type", string(base.JSON))
	}
	applyServiceToken(req)
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	start := time.Now()
	resp, doErr := client.httpClient.Do(req)
	observeExporterCall(client.service, method, endpoint, resp, doErr, time.Since(start))
	return resp, redactEndpointError(doErr, endpoint)
}

func executeHTTPRequest(
	client *Client,
	method base.Method,
	endpoint base.Endpoint,
	payload []byte,
) (*http.Response, error) {
	if mgr := connectivity.Global(); mgr != nil {
		if !mgr.IsReady(string(client.service)) {
			return nil, fmt.Errorf(string(constants.ErrConnectivityServiceNotReady), client.service)
		}
	}

	return doHTTPRequest(client, method, endpoint, payload, nil)
}

func marshalToJSON(payload any) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf(string(errors.ErrRestMarshalPayload), err)
	}
	return data, nil
}

func readResponseBody(resp *http.Response) ([]byte, error) {
	defer responseutils.CloseResponseBody(resp)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf(string(errors.ErrRestReadResponseBody), err)
	}
	return body, nil
}

func parseSingleResponse[T any](resp *http.Response) (*T, error) {
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(string(constants.ErrStatus), resp.StatusCode)
	}

	body, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}

	var data singleDataResponse[T]

	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf(string(errors.ErrRestUnmarshalResponseToGeneric), err)
	}

	return &data.Data, nil
}

func parseRawJSONResponse[T any](resp *http.Response) (*T, error) {
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(string(constants.ErrStatus), resp.StatusCode)
	}

	body, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}

	var data T
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf(string(errors.ErrRestUnmarshalResponseToGeneric), err)
	}

	return &data, nil
}

func parseListResponse[T any](resp *http.Response) ([]T, error) {
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(string(constants.ErrStatus), resp.StatusCode)
	}

	body, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}

	var data listDataResponse[T]

	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf(string(errors.ErrRestUnmarshalResponseToGeneric), err)
	}

	return data.Data.Items, nil
}

func parseGenericResponseSlice(resp *http.Response) ([]response.GenericResponse, error) {
	if resp.StatusCode >= constants.HTTPErrorCode {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf(string(constants.HTTPStatus), resp.StatusCode, string(body))
	}

	body, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}

	var apiResponses []response.GenericResponse
	if err := json.Unmarshal(body, &apiResponses); err != nil {
		return nil, fmt.Errorf(string(errors.ErrRestUnmarshalResponseToGeneric), err)
	}

	return apiResponses, nil
}

// doHTTPRequest already records every call it observes, so logging the same
// failure again here would report it twice.
func errorResponse(service base.Service, message string, err error) *response.GenericResponse {
	if isExporterDurationLogActive(service) {
		return response.NewGenericResponse(
			globalshared.StatusInternalServerError, response.OperationError, nil, message,
		)
	}
	return CreateErrorResponse(message, err)
}

func CreateErrorResponse(message string, err error) *response.GenericResponse {
	return responseutils.LogAndReturnResponse(
		globalshared.StatusInternalServerError,
		response.OperationError,
		message,
		nil,
		err,
	)
}
