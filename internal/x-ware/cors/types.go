package cors

const (
	EnvAllowedOrigins     = "CORS_ALLOWED_ORIGINS"
	allowedOriginsDivider = ","
	anyOrigin             = "*"
)

var Methods = []string{
	"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS",
}

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
}
