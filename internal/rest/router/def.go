package router

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/plsyro/rest-pkg/base"
)

type Route struct {
	Name       string
	Method     string
	Pattern    string
	HandleFunc http.HandlerFunc
}

func NewRouter(routes []Route) *mux.Router {
	router := mux.NewRouter().StrictSlash(true)
	for _, route := range routes {
		handler := route.HandleFunc
		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(handler)
	}
	return router
}

func CreateRoute(method base.Method, endpoint base.Endpoint, handlerFunc any) Route {
	return Route{
		Method:     string(method),
		Pattern:    fmt.Sprintf("/%s/%s", base.V1, endpoint),
		HandleFunc: http.HandlerFunc(handlerFunc.(func(w http.ResponseWriter, r *http.Request))),
	}
}
