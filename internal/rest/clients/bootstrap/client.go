package bootstrap

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/telark/telark/internal/data/resources/telarkconfig"
	"github.com/telark/telark/internal/rest/base"
	"github.com/telark/telark/internal/rest/clients/shared"
	"github.com/telark/telark/internal/rest/constants"
	eps "github.com/telark/telark/internal/rest/endpoints/bootstrap"
	"github.com/telark/telark/internal/rest/response"
)

type Client struct {
	*shared.Client
}

func NewClient() *Client {
	return &Client{Client: shared.New(base.Exporter)}
}

func (c *Client) Status() (*telarkconfig.BootstrapStatus, error) {
	return shared.GetTypedNoCache[telarkconfig.BootstrapStatus](c.Client, eps.GetBootstrap)
}

func (c *Client) Open(email string) *response.GenericResponse {
	return shared.ExecuteRequestWithHeaders(c.Client, base.Post, eps.OpenRequest, eps.OpenBody{Email: email}, nil)
}

func (c *Client) Approve(id string) *response.GenericResponse {
	return c.post(id, eps.ApproveRequest, nil)
}

func (c *Client) Cancel(id, reason string) *response.GenericResponse {
	return c.post(id, eps.CancelRequest, eps.CancelBody{Reason: reason})
}

func (c *Client) Complete(id string, body eps.CompleteBody) *response.GenericResponse {
	return c.post(id, eps.CompleteRequest, body)
}

func (c *Client) post(id string, endpoint base.Endpoint, body any) *response.GenericResponse {
	byID := c.WithParams(map[string]string{constants.IDParam: id})
	return shared.ExecuteRequestWithHeaders(byID, base.Post, endpoint, body, nil)
}

// The request a 200 answered with; any other answer is an error carrying the exporter's status and message.
func Result(resp *response.GenericResponse) (*eps.RequestResult, error) {
	if resp.Status != http.StatusOK {
		return nil, fmt.Errorf(string(constants.ErrUnexpectedStatus), resp.Status, resp.Message)
	}
	raw, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, err
	}
	var result eps.RequestResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
