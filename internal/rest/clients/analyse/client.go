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

type Client struct {
	sharedClient *shared.Client
}

func NewClient() *Client {
	return &Client{
		sharedClient: shared.NewClient(),
	}
}

func (c *Client) StartAnalyse() error {
	request := requestUtils.CreateGenericRequest(base.POST, base.CONFIGURATOR, base.V1, analyseEndpoints.START_ANALYSE)
	requestURL, err := request.GenerateURL()
	if err != nil {
		return fmt.Errorf(string(constants.ERROR_FAILED_GENERATE_REQUEST_URL), err)
	}

	response, err := http.Post(requestURL, constants.CONTENT_TYPE_JSON, nil)
	if err != nil {
		return fmt.Errorf(string(constants.ERROR_FAILED_SEND_POST_REQUEST), err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf(string(constants.ERROR_FAILED_READ_RESPONSE_BODY), err)
	}

	var apiResponses []restResponse.GenericResponse
	if err := json.Unmarshal(body, &apiResponses); err != nil {
		return fmt.Errorf(string(constants.ERROR_FAILED_PARSE_API_RESPONSE), err)
	}

	for i, resp := range apiResponses {
		if resp.Status != constants.HTTP_STATUS_ACCEPTED {
			return fmt.Errorf(string(constants.ERROR_RESPONSE_NOT_202_STATUS), i, resp.Status)
		}
	}

	return nil
}
