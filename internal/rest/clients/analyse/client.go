package analyse

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/plsyro/data-pkg/errors"
	globalShared "github.com/plsyro/data-pkg/shared"
	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/clients/shared"
	"github.com/plsyro/rest-pkg/constants"
	analyseEndpoints "github.com/plsyro/rest-pkg/endpoints/analyse"
	restResponse "github.com/plsyro/rest-pkg/response"
	requestUtils "github.com/plsyro/rest-pkg/utils/request"
	responseUtils "github.com/plsyro/rest-pkg/utils/response"
)

type Client struct {
	*shared.Client
}

func NewClient() *Client {
	return &Client{
		Client: shared.New(base.Configurator),
	}
}

func (c *Client) StartAnalyse() error {
	request := requestUtils.CreateGenericRequest(base.Post, base.Configurator, base.V1, analyseEndpoints.StartAnalyse)
	requestURL, err := request.GenerateURL()
	if err != nil {
		return fmt.Errorf(string(constants.ErrFailedToGenerateRequestURL), err)
	}

	//nolint:gosec // URL is generated from trusted request object
	response, err := http.Post(requestURL, string(base.JSON), nil) //nolint:bodyclose // defer responseUtils.CloseResponseBody handles closing
	if err != nil {
		return fmt.Errorf(string(constants.ErrFailedToSendPostRequest), err)
	}
	defer responseUtils.CloseResponseBody(response)

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf(string(errors.ErrRestReadResponseBody), err)
	}

	var apiResponses []restResponse.GenericResponse
	if err := json.Unmarshal(body, &apiResponses); err != nil {
		return fmt.Errorf(string(errors.ErrRestUnmarshalResponseToGeneric), err)
	}

	for _, resp := range apiResponses {
		if resp.Status != globalShared.StatusOK {
			return fmt.Errorf(string(constants.ErrUnexpectedStatus), resp.Status, resp.Message)
		}
	}

	return nil
}
