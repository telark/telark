package constants

// Shared by the authz middleware that enforces these headers and the REST
// clients that send them. They live here because neither package may import
// the other.
const (
	HeaderSessionToken = "X-Session-Token"
	HeaderServiceToken = "X-Service-Token"
	HeaderUserID       = "X-User-ID"
	HeaderUsername     = "X-Username"
	HeaderEmail        = "X-Email"
	HeaderCredentialID = "X-Credential-ID"
)

const EnvServiceToken = "TELARK_SERVICE_TOKEN"
