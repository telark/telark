package request

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

func ParseRequestBody(r *http.Request, action string, checkEmptyBody bool) (map[string]interface{}, error) {
	// Skip body parsing for "get" and "list" actions
	if action == "get" || action == "list" {
		return nil, nil
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB limit
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	// Optional empty body check
	if checkEmptyBody && len(body) == 0 {
		return nil, errors.New("request body is empty")
	}

	var spec map[string]interface{}
	if err := json.Unmarshal(body, &spec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	return spec, nil
}
