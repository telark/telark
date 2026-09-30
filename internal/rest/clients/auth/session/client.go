package session

import (
	"net/url"

	authdata "github.com/telark/telark/internal/data/auth"
	dataconstants "github.com/telark/telark/internal/data/constants"
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
	return &Client{
		Client: shared.New(base.Exporter),
	}
}

func (c *Client) withUser(userID string) *shared.Client {
	return c.WithParams(map[string]string{constants.UserIDParam: userID})
}

func listByUser(userID string) base.Endpoint {
	query := url.Values{eps.QuerySessionUser: []string{userID}}
	return base.Endpoint(string(eps.GetAllSessionsByUser) + constants.QuerySeparator + query.Encode())
}

// Only the session name leaves this process: the raw token reaches neither a URL
// nor the exporter, which resolves the self session from the name in the header.
func selfHeaders(token string) map[string]string {
	return map[string]string{dataconstants.HeaderSessionToken: authdata.SessionRef(token)}
}

func (c *Client) CreateSessionByUser(userID string, session *authdata.Session) *response.GenericResponse {
	return c.withUser(userID).Create(eps.CreateSessionByUser, session)
}

func (c *Client) GetAllSessionsByUser(userID string) ([]*authdata.Session, error) {
	return shared.GetListTyped[*authdata.Session](c.Client, listByUser(userID))
}

// Session names, usable as the token argument of the self calls.
func (c *Client) ListSessionRefsByUser(userID string) ([]string, error) {
	refs, err := shared.GetListTyped[sessionRef](c.Client, listByUser(userID))
	if err != nil {
		return nil, err
	}
	names := make([]string, constants.EmptySliceLength, len(refs))
	for _, ref := range refs {
		names = append(names, ref.Metadata.Name)
	}
	return names, nil
}

func (c *Client) GetSessionByToken(token string) (*authdata.Session, error) {
	headers := selfHeaders(token)
	headers[constants.HeaderCacheControl] = constants.CacheControlNoCache
	return shared.GetWithHeaders[authdata.Session](c.Client, eps.GetSelfSession, headers)
}

func (c *Client) PatchSessionByToken(token string, body map[string]any) *response.GenericResponse {
	return shared.ExecuteRequestWithHeaders(c.Client, base.Patch, eps.PatchSelfSession, body, selfHeaders(token))
}

func (c *Client) DeleteSessionByToken(token string) *response.GenericResponse {
	return shared.ExecuteRequestWithHeaders(c.Client, base.Delete, eps.DeleteSelfSession, nil, selfHeaders(token))
}

func (c *Client) DeleteSessionByName(name string) *response.GenericResponse {
	return c.WithParams(map[string]string{constants.NameParam: name}).Delete(eps.DeleteSessionByName)
}
