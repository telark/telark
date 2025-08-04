package shared

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/plsyro/data-pkg/common"
	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/constants"
	response "github.com/plsyro/rest-pkg/response"
	requestUtils "github.com/plsyro/rest-pkg/utils/request"
	responseUtils "github.com/plsyro/rest-pkg/utils/response"
)

func buildRequestURL(client *Client, method base.Method, endpoint base.Endpoint, payload []byte) (string, error) {
	var req base.API
	if payload != nil {
		req = requestUtils.CreateGenericRequestWithPayload(method, client.service, base.V1, endpoint, payload)
	} else {
		req = requestUtils.CreateGenericRequest(method, client.service, base.V1, endpoint)
	}

	return req.GenerateURL()
}

func executeHTTPRequest(client *Client, method base.Method, endpoint base.Endpoint, payload []byte) (*http.Response, error) {
	url, err := buildRequestURL(client, method, endpoint, payload)
	if err != nil {
		return nil, wrapError(string(constants.ErrFailedToGenerateRequestURL), err)
	}

	req, err := http.NewRequest(string(method), url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, wrapError(string(constants.ErrFailedToCreateHTTPRequest), err)
	}

	if payload != nil {
		req.Header.Set("Content-Type", string(base.JSON))
	}

	return client.httpClient.Do(req)
}

func marshalToJSON(payload any) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, wrapError(string(errors.ERROR_REST_MARSHALL_PAYLOAD), err)
	}
	return data, nil
}

func readResponseBody(resp *http.Response) ([]byte, error) {
	defer responseUtils.CloseResponseBody(resp)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, wrapError(string(errors.ERROR_REST_READ_RESPONSE_BODY), err)
	}
	return body, nil
}

func unmarshalJSON[T any](data []byte) (*T, error) {
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, wrapError(string(errors.ERROR_REST_UNMARSHALL_RESPONSE_TO_GENERIC), err)
	}
	return &result, nil
}

func parseSingleResponse[T any](resp *http.Response) (*T, error) {
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}

	return unmarshalJSON[T](body)
}

func parseListResponse[T any](resp *http.Response) ([]T, error) {
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, err := readResponseBody(resp)
	if err != nil {
		return nil, err
	}

	var apiResp struct {
		Data struct {
			Items []T `json:"items"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, wrapError(string(errors.ERROR_REST_UNMARSHALL_RESPONSE_TO_GENERIC), err)
	}

	return apiResp.Data.Items, nil
}

func substituteEndpointName(endpoint base.Endpoint, name string) base.Endpoint {
	return base.Endpoint(strings.Replace(string(endpoint), string(constants.EndpointNamePlaceholder), name, 1))
}

func wrapError(operation string, err error) error {
	return fmt.Errorf("failed to %s: %w", operation, err)
}

func createErrorResponse(message string, err error) *response.GenericResponse {
	return responseUtils.LogAndReturnResponse(
		common.STATUS_INTERNAL_SERVER_ERROR,
		response.OPERATION_ERROR,
		message,
		nil,
		err,
	)
}
