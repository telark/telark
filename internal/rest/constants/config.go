package constants

import (
	"time"

	"github.com/telark/data/errors"
)

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
	ErrEndpointCall                  errors.Error = "%s %s: %v"
	HTTPStatus                       errors.Error = "HTTP %d: %s"
	ErrRequestBodyTooLarge           errors.Error = "request body too large"
	DefaultHTTPPort                               = 80
	DefaultHTTPSPort                              = 443
	DefaultTimeout                                = 30
	HTTPErrorCode                                 = 400
	HTTPServerErrorCode                           = 500
	PeerServerErrorMessage                        = "upstream service error"
	ReplaceCount                                  = 1
	EmptySliceLength                              = 0
	FirstIndex                                    = 0
	SecondIndex                                   = 1
	EmptyString                                   = ""
	OmitEmpty                                     = "omitempty"
	QuerySeparator                                = "?"
	HeaderCacheControl                            = "Cache-Control"
	CacheControlNoCache                           = "no-cache"
	NameParam                                     = "{name}"
	UserIDParam                                   = "{userId}"
	IDParam                                       = "{id}"
	CredentialIDParam                             = "{credentialId}"
	TypeParam                                     = "{type}"
	UsernameParam                                 = "{username}"
	EmailParam                                    = "{email}"
	FieldFinalizerName                            = "name"
	ConnectivityKeyPrefix                         = "connectivity:service:"
	ConnectivityValueReady                        = "1"
	ConnectivityValueNotReady                     = "0"
	ConnectivitySyncIntervalSeconds               = 1
	ConnectivityKeyTTLSeconds                     = 3
	ConnectivityRedisOpTimeoutMillis              = 200
	ErrStatus                        errors.Error = "status: %d"
	ErrConnectivityServiceNotReady   errors.Error = "service not ready: %s"
	ErrConfigReadFailed              errors.Error = "failed to read telark config before patch: %v"
	ErrConfigResponseData            errors.Error = "invalid telark config response data"
	ErrConfigMetadataMissing         errors.Error = "telark config metadata is missing"
	ErrConfigVersionMissing          errors.Error = "telark config resourceVersion is missing"
	ErrConfigPatchRetryFailure       errors.Error = "telark config patch conflict after %d retries"
	MetadataField                                 = "metadata"
	SpecField                                     = "spec"
	ResourceVersionField                          = "resourceVersion"
	ConfigConflictStatus                          = 409
	ConfigPatchRetryAttempts                      = 3
	ConfigPatchRetryBackoff                       = 200 * time.Millisecond
	ConfigConflictMessageOne                      = "the object has been modified"
	ConfigConflictMessageTwo                      = "operation cannot be fulfilled"

	EnvExporterDurationLogEnabled      = "REST_EXPORTER_DURATION_LOG_ENABLED"
	EnvExporterDurationLogDedupSec     = "REST_EXPORTER_DURATION_LOG_DEDUP_SEC"
	DefaultExporterDurationLogDedupSec = 10
	NoStatusCode                       = -1
	MillisecondsPerSecond              = 1000

	LogExporterCallError errors.Error = "exporter call error service=%s method=%s endpoint=%s status=%d" +
		" elapsedMs=%d suppressedSinceLast=%d err=%v"
)
