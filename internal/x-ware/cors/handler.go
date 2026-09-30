package cors

import (
	"net/http"
	"os"
	"strings"

	"github.com/rs/cors"
	"github.com/telark/x-ware/constants"
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
		AllowCredentials: true,
		Debug:            false,
	})
	return corsHandler.Handler
}

func AllowedOrigins() []string {
	var origins []string
	for origin := range strings.SplitSeq(os.Getenv(EnvAllowedOrigins), allowedOriginsDivider) {
		// With credentials, rs/cors answers "*" by reflecting whatever origin asks.
		if trimmed := strings.TrimSpace(origin); trimmed != constants.EmptyString && trimmed != anyOrigin {
			origins = append(origins, trimmed)
		}
	}
	return origins
}
