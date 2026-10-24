package cors

import (
	"net/http"
	"os"
	"strings"

	"github.com/rs/cors"
	"github.com/telark/telark/internal/x-ware/constants"
)

// No configured origin means same-origin only: the handler passes requests
// through untouched and never answers a cross-origin preflight.
func NewCORS() func(http.Handler) http.Handler {
	origins := AllowedOrigins()
	if len(origins) == constants.EmptySliceLength {
		return func(next http.Handler) http.Handler { return next }
	}
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   Methods,
		AllowedHeaders:   Headers,
		ExposedHeaders:   exposedHeaders,
		AllowCredentials: true,
		Debug:            false,
	})
	return corsHandler.Handler
}

func AllowedOrigins() []string {
	var origins []string
	for origin := range strings.SplitSeq(os.Getenv(EnvAllowedOrigins), allowedOriginsDivider) {
		// Exact origins only: with credentials, rs/cors reflects whatever origin asks for "*", and treats any entry
		// holding one ("https://*") as a pattern.
		if trimmed := strings.TrimSpace(origin); trimmed != constants.EmptyString && !strings.Contains(trimmed, anyOrigin) {
			origins = append(origins, trimmed)
		}
	}
	return origins
}
