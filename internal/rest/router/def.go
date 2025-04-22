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

func CreateRoute(method base.Method, endpoint base.Endpoint, handlerFunc interface{}) Route {
	return Route{
		Method:     string(method),
		Pattern:    fmt.Sprintf("/%s/%s", base.V1, endpoint),
		HandleFunc: http.HandlerFunc(handlerFunc.(func(w http.ResponseWriter, r *http.Request))),
	}
}
