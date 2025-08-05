package analyze

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/plsyro/data/errors"
	globalshared "github.com/plsyro/data/shared"
	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/clients/shared"
	"github.com/plsyro/rest/constants"
	eps "github.com/plsyro/rest/endpoints/analyze"
	restresponse "github.com/plsyro/rest/response"
	requestutils "github.com/plsyro/rest/utils/request"
	responseutils "github.com/plsyro/rest/utils/response"
)

type Client struct {
	*shared.Client
}

func NewClient() *Client {
	return &Client{
		Client: shared.New(base.Configurator),
	}
}

func (*Client) Startanalyze() error {
	request := requestutils.CreateGenericRequest(
		base.Post,
		base.Configurator,
		base.V1,
		eps.Startanalyze,
	)
	requestURL, err := request.GenerateURL()
	if err != nil {
		return fmt.Errorf(string(constants.ErrFailedToGenerateRequestURL), err)
	}

	//nolint:gosec // URL is generated from trusted request object
	// defer responseutils.CloseResponseBody handles closing
	response, err := http.Post(requestURL, string(base.JSON), nil) //nolint:bodyclose
	if err != nil {
		return fmt.Errorf(string(constants.ErrFailedToSendPostRequest), err)
	}
	defer responseutils.CloseResponseBody(response)

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf(string(errors.ErrRestReadResponseBody), err)
	}

	var apiResponses []restresponse.GenericResponse
	if err := json.Unmarshal(body, &apiResponses); err != nil {
		return fmt.Errorf(string(errors.ErrRestUnmarshalResponseToGeneric), err)
	}

	for _, resp := range apiResponses {
		if resp.Status != globalshared.StatusOK {
			return fmt.Errorf(string(constants.ErrUnexpectedStatus), resp.Status, resp.Message)
		}
	}

	return nil
}
