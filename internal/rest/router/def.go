package router

import (
	"net/http"

	"github.com/gorilla/mux"
)

type Route struct {
	Name       string
	Method     string
	Pattern    string
	HandleFunc http.HandlerFunc
}

func NewRouter(Routes []Route) *mux.Router {

	router := mux.NewRouter().StrictSlash(true)
	for _, route := range Routes {
		handler := route.HandleFunc
		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(handler)
	}

	return router
}
