package shared

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/telark/telark/internal/rest/base"
	"github.com/telark/telark/internal/rest/clients/shared"
	"github.com/telark/telark/internal/rest/clients/users"
	eps "github.com/telark/telark/internal/rest/endpoints/users"
	"github.com/telark/telark/internal/rest/router"
	requestutils "github.com/telark/telark/internal/rest/utils/request"
)

const (
	emailVar       = "email"
	byEmailRoute   = "by-email"
	listRoute      = "list"
	redirectTarget = "/api/v1/users"
)

type routeLog struct {
	mu     sync.Mutex
	hits   []string
	emails []string
}

func (l *routeLog) record(route, email string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hits = append(l.hits, route)
	if email != emptyBody {
		l.emails = append(l.emails, email)
	}
}

func usersServer(t *testing.T, log *routeLog) *httptest.Server {
	t.Helper()
	handler := router.NewRouter([]router.Route{
		router.CreateRoute(base.Get, eps.GetUserByEmail, func(w http.ResponseWriter, r *http.Request) {
			email, err := requestutils.ReadPathParam(r, emailVar)
			if err != nil {
				t.Errorf("read email: %v", err)
			}
			log.record(byEmailRoute, email)
			w.WriteHeader(http.StatusNotFound)
		}),
		router.CreateRoute(base.Get, eps.GetAllUsers, func(w http.ResponseWriter, _ *http.Request) {
			log.record(listRoute, emptyBody)
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte(itemsBody)); err != nil {
				t.Errorf("write response: %v", err)
			}
		}),
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func TestPathParamsCannotLeaveTheirRoute(t *testing.T) {
	cases := []struct {
		name  string
		email string
	}{
		{"dot segments", "a/../../get"},
		{"pre-encoded dot segments", "a%2F%2E%2E%2F%2E%2E%2Fget?@example.com"},
		{"query injection", "a?limit=1@example.com"},
		{"fragment injection", "a#frag@example.com"},
		{"bare parent", ".."},
		{"plain", "jane.doe@example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			log := &routeLog{}
			server := usersServer(t, log)
			client := users.NewClient()
			client.GetHTTPClient().Transport = forwardTo(server)

			_, _ = client.GetUserByEmail(tc.email)

			for _, hit := range log.hits {
				if hit != byEmailRoute {
					t.Fatalf("value %q reached route %q", tc.email, hit)
				}
			}
			for _, got := range log.emails {
				if got != tc.email {
					t.Fatalf("route received %q, want the value verbatim %q", got, tc.email)
				}
			}
		})
	}
}

func TestRedirectsAreNotFollowed(t *testing.T) {
	var targetHits int
	var mu sync.Mutex
	mux := http.NewServeMux()
	mux.HandleFunc(redirectTarget, func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		targetHits++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget, http.StatusFound)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := shared.New(base.Exporter)
	client.GetHTTPClient().Transport = forwardTo(server)

	if _, err := client.Get(eps.GetAllUsers + "x"); err == nil {
		t.Fatal("a redirect must surface as a failed call")
	}
	if targetHits != noClose {
		t.Fatalf("redirect target was requested %d times, want 0", targetHits)
	}
}
