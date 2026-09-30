package passkey

import (
	authdata "github.com/telark/telark/internal/data/auth"
	"github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/rest/base"
	"github.com/telark/telark/internal/rest/clients/shared"
	"github.com/telark/telark/internal/rest/constants"
	eps "github.com/telark/telark/internal/rest/endpoints/auth"
	restmapper "github.com/telark/telark/internal/rest/mappers"
	"github.com/telark/telark/internal/rest/response"
)

const HeaderUserID = "X-User-ID"

type Client struct {
	*shared.Client
}

func NewClient() *Client {
	return &Client{
		Client: shared.New(base.Exporter),
	}
}

func (c *Client) withCredential(credentialID string) *shared.Client {
	return c.WithParams(map[string]string{constants.CredentialIDParam: credentialID})
}

func (c *Client) CreatePasskeyByUser(userID string, passkey *authdata.Passkey) *response.GenericResponse {
	mappedPayload, err := restmapper.MapToJSONPayload(passkey)
	if err != nil {
		return shared.CreateErrorResponse(string(errors.ErrRestMarshalPayload), err)
	}
	return shared.ExecuteRequestWithHeaders(c.Client, base.Post,
		eps.CreateInternalPasskeyByUser,
		mappedPayload,
		userHeaders(userID),
	)
}

func (c *Client) GetAllPasskeysByUser(userID string) ([]*authdata.Passkey, error) {
	return shared.GetListWithHeaders[*authdata.Passkey](c.Client, eps.GetAllInternalPasskeysByUser, userHeaders(userID))
}

func (c *Client) GetPasskeyByUserAndCredentialID(userID string, credentialID string) (*authdata.Passkey, error) {
	return shared.GetWithHeaders[authdata.Passkey](
		c.withCredential(credentialID),
		eps.GetInternalPasskeyByUserAndCredentialID,
		userHeaders(userID),
	)
}

func (c *Client) PatchPasskeyByUserAndCredentialID(
	userID string, credentialID string, body map[string]any,
) *response.GenericResponse {
	return shared.ExecuteRequestWithHeaders(
		c.withCredential(credentialID), base.Patch, eps.PatchInternalPasskeyByUserAndCredentialID, body, userHeaders(userID),
	)
}

func (c *Client) DeletePasskeyByUserAndCredentialID(
	userID string,
	credentialID string,
	forceLastDelete bool,
) *response.GenericResponse {
	body := map[string]any{}
	if forceLastDelete {
		body["forceLastDelete"] = true
	}
	return shared.ExecuteRequestWithHeaders(
		c.withCredential(credentialID), base.Delete,
		eps.DeleteInternalPasskeyByUserAndCredentialID,
		body,
		userHeaders(userID),
	)
}

func userHeaders(userID string) map[string]string {
	return map[string]string{HeaderUserID: userID}
}
