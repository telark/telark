package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/router"
)

func TestNewRouter(t *testing.T) {
	routes := []router.Route{
		router.CreateRoute(base.Get, "hello", func(w http.ResponseWriter, r *http.Request) {
			if _, err := w.Write([]byte("Hello, World!")); err != nil {
				t.Errorf("failed to write response: %v", err)
			}
		}),
	}

	router := router.NewRouter(routes)
	req, err := http.NewRequest("GET", "/api/v1/hello", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expected := `Hello, World!`
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}

	t.Log("TestNewRouter passed: Router correctly handled the request and returned the expected response.")
}

func TestCreateRoute(t *testing.T) {
	route := router.CreateRoute(base.Get, "test", func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("Test Route")); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	})

	if route.Method != string(base.Get) {
		t.Errorf("CreateRoute() method = %v, want %v", route.Method, base.Get)
	}

	if route.Pattern != "/api/v1/test" {
		t.Errorf("CreateRoute() pattern = %v, want %v", route.Pattern, "/api/v1/test")
	}

	t.Log("TestCreateRoute passed: Route was created with the correct method and pattern.")
}
