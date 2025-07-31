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

// DoRequest sends an HTTP request and unmarshals the response into the provided type T.
func DoRequest[T any](req *http.Request) (*T, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(string(constants.ERROR_UNEXPECTED_STATUS), resp.StatusCode)
	}

	var apiResp APIResponse
	apiResp.Data = new(T)
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, err
	}

	// Unmarshal Data into T
	dataBytes, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, err
	}
	var result T
	if err := json.Unmarshal(dataBytes, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// DoRequestList handles list responses (where items are under a key in Data)
func DoRequestList[T any](req *http.Request, itemsKey string) ([]T, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(string(constants.ERROR_UNEXPECTED_STATUS), resp.StatusCode)
	}

	var apiResp map[string]any
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, err
	}

	data, ok := apiResp["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf(string(constants.ERROR_UNEXPECTED_DATA_FIELD_FORMAT))
	}
	itemsRaw, ok := data[itemsKey]
	if !ok {
		return nil, fmt.Errorf(string(constants.ERROR_ITEMS_FIELD_MISSING))
	}
	itemsJSON, err := json.Marshal(itemsRaw)
	if err != nil {
		return nil, err
	}
	var items []T
	if err := json.Unmarshal(itemsJSON, &items); err != nil {
		return nil, err
	}
	return items, nil
}
