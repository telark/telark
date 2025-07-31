package analyse

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/clients/shared"
	"github.com/plsyro/rest-pkg/constants"
	analyseEndpoints "github.com/plsyro/rest-pkg/endpoints/analyse"
	restResponse "github.com/plsyro/rest-pkg/response"
	requestUtils "github.com/plsyro/rest-pkg/utils/request"
)

// Client provides a unified interface for analyse operations
type Client struct {
	sharedClient *shared.Client
}

// NewClient creates a new Client instance
func NewClient() *Client {
	return &Client{
		sharedClient: shared.NewClient(),
	}
}

// StartAnalyse starts the analysis process
func (c *Client) StartAnalyse() error {
	// Prepare Request
	request := requestUtils.CreateGenericRequest(base.POST, base.CONFIGURATOR, base.V1, analyseEndpoints.START_ANALYSE)

	// Generate Request URL
	requestURL, err := request.GenerateURL()
	if err != nil {
		return fmt.Errorf(string(constants.ERROR_FAILED_GENERATE_REQUEST_URL), err)
	}

	// Send POST Request
	response, err := http.Post(requestURL, constants.CONTENT_TYPE_JSON, nil)
	if err != nil {
		return fmt.Errorf(string(constants.ERROR_FAILED_SEND_POST_REQUEST), err)
	}
	defer response.Body.Close()

	// Read Response Body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf(string(constants.ERROR_FAILED_READ_RESPONSE_BODY), err)
	}

	// Parse JSON into a slice of GenericResponse
	var apiResponses []restResponse.GenericResponse
	if err := json.Unmarshal(body, &apiResponses); err != nil {
		return fmt.Errorf(string(constants.ERROR_FAILED_PARSE_API_RESPONSE), err)
	}

	// Check that all responses have status 202
	for i, resp := range apiResponses {
		if resp.Status != constants.HTTP_STATUS_ACCEPTED {
			return fmt.Errorf(string(constants.ERROR_RESPONSE_NOT_202_STATUS), i, resp.Status)
		}
	}

	return nil
}
