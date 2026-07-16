package shared

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func buildRequestURL(
	client *Client,
	method base.Method,
	endpoint base.Endpoint,
	payload []byte,
) (string, error) {
	var req base.API
	if payload != nil {
		req = requestutils.CreateGenericRequestWithPayload(
			client.service,
			base.V1,
			endpoint,
			payload,
		)
	} else {
		req = requestutils.CreateGenericRequest(
			method, client.service, base.V1, endpoint)
	}

	return req.GenerateURL()
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
	applyServiceToken(req)

	start := time.Now()
	resp, doErr := client.httpClient.Do(req)
	observeExporterCall(client.service, method, endpoint, resp, doErr, time.Since(start))
	return resp, doErr
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

func SubstituteEndpointWithParam(endpoint string, placeholder string, value string) base.Endpoint {
	return base.Endpoint(strings.Replace(endpoint, placeholder, value, constants.ReplaceCount))
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
