package request

import (
	"github.com/telark/telark/internal/data/resources/authrequest"
	"github.com/telark/telark/internal/rest/base"
	"github.com/telark/telark/internal/rest/clients/shared"
	"github.com/telark/telark/internal/rest/constants"
	eps "github.com/telark/telark/internal/rest/endpoints/auth"
	"github.com/telark/telark/internal/rest/response"
)

type Client struct {
	*shared.Client
}

func NewClient() *Client {
	return &Client{Client: shared.New(base.Exporter)}
}

// Read fresh: a cached list would hand a finished request to the next run.
func (c *Client) Pending() ([]authrequest.AuthRequest, error) {
	headers := map[string]string{constants.HeaderCacheControl: constants.CacheControlNoCache}
	return shared.GetListWithHeaders[authrequest.AuthRequest](c.Client, eps.GetPendingAuthRequests, headers)
}

func (c *Client) SetStatus(id string, status authrequest.Status) *response.GenericResponse {
	byID := c.WithParams(map[string]string{constants.IDParam: id})
	return shared.ExecuteRequestWithHeaders(byID, base.Patch, eps.PatchAuthRequestStatus, status, nil)
}
