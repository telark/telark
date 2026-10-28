package constants

// Shared by the authz middleware that enforces these headers and the REST clients that send
// them, since neither package may import the other.
const (
	HeaderSessionToken = "X-Session-Token"
	HeaderServiceToken = "X-Service-Token"
	HeaderUserID       = "X-User-ID"
	HeaderUsername     = "X-Username"
	HeaderEmail        = "X-Email"
	HeaderCredentialID = "X-Credential-ID"
	// Any value: a page on another origin can't add it without a CORS preflight that fails.
	HeaderRequestedWith = "X-Requested-With"
	// The browser's session, HttpOnly: no script reads it, and x-ware turns it into X-Session-Token.
	CookieSession = "telark_session"
	// Browsers keep a __Host- cookie only if it is Secure, host-only and on Path=/, so no sibling host can plant it.
	CookieSessionHostPrefix = "__Host-"
)

const (
	EnvServiceToken = "TELARK_SERVICE_TOKEN"
	// The chart sets it when every passkey origin is https: the session cookie then takes CookieSessionHostPrefix.
	EnvSessionCookieHostPrefix = "SESSION_COOKIE_HOST_PREFIX"
)
