package constants

import "github.com/plsyro/data/errors"

const (
	ErrFailedToGenerateRequestURL    errors.Error = "failed to generate request URL: %v"
	ErrFailedToSendPostRequest       errors.Error = "failed to send POST request: %v"
	ErrUnexpectedStatus              errors.Error = "unexpected status: %d: %s"
	ErrFailedToCreateHTTPRequest     errors.Error = "failed to create HTTP request: %v"
	ErrFailedToCloseResponseBody     errors.Error = "failed to close response body: %v"
	ErrSchemaIsRequired              errors.Error = "schema is required"
	ErrServiceIsRequired             errors.Error = "service is required"
	ErrVersionIsRequired             errors.Error = "version is required"
	ErrEndpointIsRequired            errors.Error = "endpoint is required"
	ErrValidationFailed              errors.Error = "validation failed: %v"
	HTTPStatus                       errors.Error = "HTTP %d: %s"
	DefaultHTTPPort                               = 80
	DefaultHTTPSPort                              = 443
	DefaultTimeout                                = 30
	HTTPErrorCode                                 = 400
	ReplaceCount                                  = 1
	EmptySliceLength                              = 0
	FirstIndex                                    = 0
	SecondIndex                                   = 1
	EmptyString                                   = ""
	OmitEmpty                                     = "omitempty"
	NameParam                                     = "{name}"
	UserIDParam                                   = "{userId}"
	TokenParam                                    = "{token}"
	IDParam                                       = "{id}"
	UsernameParam                                 = "{username}"
	ConnectivityKeyPrefix                         = "connectivity:service:"
	ConnectivityValueReady                        = "1"
	ConnectivityValueNotReady                     = "0"
	ConnectivitySyncIntervalSeconds               = 1
	ConnectivityKeyTTLSeconds                     = 3
	ConnectivityRedisOpTimeoutMillis              = 200
	ErrStatus                        errors.Error = "status: %d"
	ErrConnectivityServiceNotReady   errors.Error = "service not ready: %s"
)
