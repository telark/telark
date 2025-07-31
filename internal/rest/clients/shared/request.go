package shared

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/plsyro/rest-pkg/constants"
)

type APIResponse struct {
	Status int `json:"status"`
	Data   any `json:"data"`
}

func DoRequest[T any](req *http.Request) (*T, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_EXECUTE_REQUEST), err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_READ_RESPONSE_BODY_GENERIC), err)
	}

	if resp.StatusCode != constants.HTTP_STATUS_OK {
		return nil, fmt.Errorf(string(constants.ERROR_UNEXPECTED_STATUS_CODE), resp.StatusCode)
	}

	var apiResp APIResponse
	apiResp.Data = new(T)
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_UNMARSHAL_RESPONSE), err)
	}

	dataBytes, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_MARSHAL_DATA), err)
	}
	var result T
	if err := json.Unmarshal(dataBytes, &result); err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_UNMARSHAL_DATA), err)
	}

	return &result, nil
}

func DoRequestList[T any](req *http.Request, itemsKey string) ([]T, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_EXECUTE_REQUEST), err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_READ_RESPONSE_BODY_GENERIC), err)
	}

	if resp.StatusCode != constants.HTTP_STATUS_OK {
		return nil, fmt.Errorf(string(constants.ERROR_UNEXPECTED_STATUS_CODE), resp.StatusCode)
	}

	var apiResp map[string]any
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_UNMARSHAL_RESPONSE), err)
	}

	data, ok := apiResp[constants.RESPONSE_DATA_KEY].(map[string]any)
	if !ok {
		return nil, fmt.Errorf(string(constants.ERROR_UNEXPECTED_DATA_FIELD_FORMAT))
	}
	itemsRaw, ok := data[itemsKey]
	if !ok {
		return nil, fmt.Errorf(string(constants.ERROR_ITEMS_FIELD_MISSING))
	}
	itemsJSON, err := json.Marshal(itemsRaw)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_MARSHAL_ITEMS), err)
	}
	var items []T
	if err := json.Unmarshal(itemsJSON, &items); err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_UNMARSHAL_ITEMS), err)
	}
	return items, nil
}
