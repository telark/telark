package router

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/telark/rest/base"
	"github.com/telark/rest/constants"
)

const (
	pathSeparator = "/"
	keySeparator  = " "
)

type Route struct {
	Name       string
	Method     string
	Pattern    string
	HandleFunc http.HandlerFunc
}

func NewRouter(routes []Route) *mux.Router {
	router := mux.NewRouter()
	for _, route := range routes {
		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(route.HandleFunc)
	}
	return router
}

func CreateRoute(method base.Method, endpoint base.Endpoint, handlerFunc http.HandlerFunc) Route {
	return Route{
		Method:     string(method),
		Pattern:    Pattern(endpoint),
		HandleFunc: handlerFunc,
	}
}

func Pattern(endpoint base.Endpoint) string {
	return pathSeparator + string(base.V1) + pathSeparator + string(endpoint)
}

func Key(method base.Method, endpoint base.Endpoint) string {
	return buildKey(string(method), Pattern(endpoint))
}

// Only meaningful once mux has matched: a middleware relying on it must be
// registered with Router.Use rather than wrapped around the router.
func KeyFromRequest(r *http.Request) string {
	route := mux.CurrentRoute(r)
	if route == nil {
		return constants.EmptyString
	}

	template, err := route.GetPathTemplate()
	if err != nil {
		return constants.EmptyString
	}

	return buildKey(r.Method, template)
}

// Both keys are built here from the same pattern the router registers, so a
// lookup table cannot drift out of step with the routes it describes.
func buildKey(method, pattern string) string {
	return method + keySeparator + pattern
}
