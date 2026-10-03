package cors

const (
	EnvAllowedOrigins     = "CORS_ALLOWED_ORIGINS"
	allowedOriginsDivider = ","
	anyOrigin             = "*"
)

var Methods = []string{
	"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS",
}

// Not CORS-safelisted: without them the browser hides the delay a shed request carries
// and the ETag the UI revalidates insights with.
var exposedHeaders = []string{"Retry-After", "ETag"}

var Headers = []string{
	"Content-Type",
	"X-Silent-404",
	"X-Silent-Network",
	"X-Session-Token",
	"X-Credential-ID",
	"X-Device-Name",
	"X-Device-Type",
	"X-Username",
	"X-Email",
	"Accept",
	"Origin",
	"If-None-Match",
}
