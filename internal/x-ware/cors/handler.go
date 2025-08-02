package cors

import (
	"net/http"

	"github.com/rs/cors"
)

func NewCORS() func(http.Handler) http.Handler {
	corsHandler := cors.New(cors.Options{
		AllowedOrigins: Origins,
		AllowedMethods: Methods,
		AllowedHeaders: Headers,
	})
	return corsHandler.Handler
}
