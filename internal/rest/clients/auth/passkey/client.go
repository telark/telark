package passkey

import (
	authdata "github.com/telark/data/auth"
	"github.com/telark/data/errors"
	"github.com/telark/rest/base"
	"github.com/telark/rest/clients/shared"
	eps "github.com/telark/rest/endpoints/auth"
	restmapper "github.com/telark/rest/mappers"
	"github.com/telark/rest/response"
)

const (
	HeaderUserID       = "X-User-ID"
	HeaderCredentialID = "X-Credential-ID"
)

type Client struct {
	*shared.Client
}

func NewClient() *Client {
	return &Client{
		Client: shared.New(base.Exporter),
	}
}

func (c *Client) CreatePasskeyByUser(userID string, passkey *authdata.UserPasskey) *response.GenericResponse {
	mappedPayload, err := restmapper.MapToJSONPayload(passkey)
	if err != nil {
		return shared.CreateErrorResponse(string(errors.ErrRestMarshalPayload), err)
	}
	headers := map[string]string{
		HeaderUserID: userID,
	}
	return shared.ExecuteRequestWithHeaders(c.Client, base.Post,
		eps.CreateInternalPasskeyByUser,
		mappedPayload,
		headers,
	)
}

func (c *Client) GetAllPasskeysByUser(userID string) ([]*authdata.UserPasskey, error) {
	headers := map[string]string{
		HeaderUserID: userID,
	}
	return shared.GetListWithHeaders[*authdata.UserPasskey](c.Client, eps.GetAllInternalPasskeysByUser, headers)
}

func (c *Client) GetPasskeyByUserAndCredentialID(userID string, credentialID string) (*authdata.UserPasskey, error) {
	headers := map[string]string{
		HeaderUserID:       userID,
		HeaderCredentialID: credentialID,
	}
	return shared.GetWithHeaders[authdata.UserPasskey](
		c.Client,
		eps.GetInternalPasskeyByUserAndCredentialID,
		headers,
	)
}

func (c *Client) PatchPasskeyByUserAndCredentialID(
	userID string, credentialID string, body map[string]any,
) *response.GenericResponse {
	headers := map[string]string{
		HeaderUserID:       userID,
		HeaderCredentialID: credentialID,
	}
	return shared.ExecuteRequestWithHeaders(
		c.Client, base.Patch, eps.PatchInternalPasskeyByUserAndCredentialID, body, headers,
	)
}

//nolint:revive
func (c *Client) DeletePasskeyByUserAndCredentialID(
	userID string,
	credentialID string,
	forceLastDelete bool,
) *response.GenericResponse {
	headers := map[string]string{
		HeaderUserID:       userID,
		HeaderCredentialID: credentialID,
	}
	body := map[string]any{}
	if forceLastDelete {
		body["forceLastDelete"] = true
	}
	return shared.ExecuteRequestWithHeaders(
		c.Client, base.Delete,
		eps.DeleteInternalPasskeyByUserAndCredentialID,
		body,
		headers,
	)
}
