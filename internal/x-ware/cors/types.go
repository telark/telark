package cors

var Origins = []string{
	"http://localhost:3000",
}

var Methods = []string{
	"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS",
}

var Headers = []string{
	"Content-Type",
	"Authorization",
	"X-Silent-404",
	"X-Silent-Network",
	"X-Session-Token",
	"X-Credential-ID",
	"X-Device-Name",
	"X-Device-Type",
	"X-Username",
	"X-Email",
	"X-User-ID",
	"Accept",
	"Origin",
}
