package analyse

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/plsyro/data-pkg/common"
	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/clients/shared"
	"github.com/plsyro/rest-pkg/constants"
	analyseEndpoints "github.com/plsyro/rest-pkg/endpoints/analyse"
	restResponse "github.com/plsyro/rest-pkg/response"
	requestUtils "github.com/plsyro/rest-pkg/utils/request"
)

type Client struct {
	*shared.Client
}

func NewClient() *Client {
	return &Client{
		Client: shared.New(base.CONFIGURATOR),
	}
}

func (c *Client) StartAnalyse() error {
	request := requestUtils.CreateGenericRequest(base.POST, base.CONFIGURATOR, base.V1, analyseEndpoints.START_ANALYSE)
	requestURL, err := request.GenerateURL()
	if err != nil {
		return fmt.Errorf(string(constants.ERROR_FAILED_GENERATE_REQUEST_URL), err)
	}

	response, err := http.Post(requestURL, string(base.JSON), nil)
	if err != nil {
		return fmt.Errorf(string(constants.ERROR_FAILED_SEND_POST_REQUEST), err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			err = fmt.Errorf(string(constants.ERROR_FAILED_CLOSE_RESPONSE_BODY), closeErr)
		}
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf(string(errors.ERROR_REST_READ_RESPONSE_BODY), err)
	}

	var apiResponses []restResponse.GenericResponse
	if err := json.Unmarshal(body, &apiResponses); err != nil {
		return fmt.Errorf(string(errors.ERROR_REST_UNMARSHALL_RESPONSE_TO_GENERIC), err)
	}

	for _, resp := range apiResponses {
		if resp.Status != common.STATUS_OK {
			return fmt.Errorf(string(constants.ERROR_UNEXPECTED_STATUS), resp.Status, resp.Message)
		}
	}

	return nil
}
