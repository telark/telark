package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	authdata "github.com/telark/telark/internal/data/auth"
	groupdata "github.com/telark/telark/internal/data/resources/group"
	roledata "github.com/telark/telark/internal/data/resources/role"
	userresource "github.com/telark/telark/internal/data/resources/user"
	restresponse "github.com/telark/telark/internal/rest/response"
	"github.com/telark/telark/services/auth/internal/clients"
	"github.com/telark/telark/services/auth/internal/constants"
	authzhandler "github.com/telark/telark/services/auth/internal/handlers/authorisation"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

// exporterStub answers the session lookup as configured and every other
// lookup (the user record) with a success.
type exporterStub struct {
	session func() (*http.Response, error)
}

func (s exporterStub) RoundTrip(r *http.Request) (*http.Response, error) {
	if strings.HasSuffix(r.URL.Path, "/api/v1/auth/sessions/self") {
		return s.session()
	}
	return envelope(http.StatusOK, userresource.User{ID: "uid"})
}

func envelope(status int, data any) (*http.Response, error) {
	body, err := json.Marshal(map[string]any{"data": data})
	if err != nil {
		return nil, err
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(body))}, nil
}

func liveSession() (*http.Response, error) {
	return envelope(http.StatusOK, authdata.Session{
		UserID:           "uid",
		ExpiresTimestamp: time.Now().UTC().Add(time.Hour).Format(constants.TimeFormatRFC3339),
	})
}

func stubExporter(t *testing.T, stub http.RoundTripper) {
	t.Helper()
	for _, c := range []*http.Client{
		clients.GetSessionClient().GetHTTPClient(),
		clients.GetUserClient().GetHTTPClient(),
		clients.GetPasskeyClient().GetHTTPClient(),
	} {
		previous := c.Transport
		c.Transport = stub
		t.Cleanup(func() { c.Transport = previous })
	}
}

// A session backend that answered "no such session" is a verdict (401); one
// that could not answer at all is an outage (503, Unavailable) and must never
// read as a logout.
func TestGetPermissionsSessionOutcomes(t *testing.T) {
	cases := []struct {
		name      string
		session   func() (*http.Response, error)
		status    int
		operation restresponse.OperationStatus
	}{
		{"transport error", func() (*http.Response, error) {
			return nil, errors.New("dial tcp: connection refused")
		}, http.StatusServiceUnavailable, restresponse.OperationUnavailable},
		{"not found", func() (*http.Response, error) {
			return envelope(http.StatusNotFound, nil)
		}, http.StatusUnauthorized, ""},
		{"success", liveSession, http.StatusOK, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stubExporter(t, exporterStub{session: c.session})
			rec := httptest.NewRecorder()
			authzhandler.GetPermissions(rec, withSession(httptest.NewRequest(http.MethodGet, "/", nil)))
			if rec.Code != c.status {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, c.status, rec.Body.String())
			}
			if c.operation == "" {
				return
			}
			var body restresponse.GenericResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body.Operation != string(c.operation) {
				t.Fatalf("operation = %q, want %q", body.Operation, c.operation)
			}
		})
	}
}

// permissionsStub serves a user holding one temporary role whose expiry does not
// parse and one role that is being deleted.
type permissionsStub struct{}

func (permissionsStub) RoundTrip(r *http.Request) (*http.Response, error) {
	deleting := "2026-01-01T00:00:00Z"
	badExpiry := "not-a-date"
	switch {
	case strings.HasSuffix(r.URL.Path, "/api/v1/auth/sessions/self"):
		return liveSession()
	case strings.Contains(r.URL.Path, "/accessroles/r-bad"):
		return envelope(http.StatusOK, roledata.AccessRole{ID: "r-bad", Status: roledata.RoleStatusActive,
			Validity: &roledata.Validity{Type: roledata.ValidityTypeTemporary, ExpiresAt: &badExpiry}})
	case strings.Contains(r.URL.Path, "/accessroles/r-gone"):
		return envelope(http.StatusOK, roledata.AccessRole{ID: "r-gone", Status: roledata.RoleStatusActive,
			DeletionTimestamp: &deleting})
	default:
		bad, gone := "r-bad", "r-gone"
		return envelope(http.StatusOK, userresource.User{ID: "uid", RoleRefs: []*string{&bad, &gone}})
	}
}

// The permissions view follows x-ware/authz: an unparsable expiry is expired and
// a role being deleted is not listed at all.
func TestGetPermissionsMatchesAuthzRules(t *testing.T) {
	stubExporter(t, permissionsStub{})
	for _, c := range []*http.Client{clients.GetAccessRoleClient().GetHTTPClient()} {
		previous := c.Transport
		c.Transport = permissionsStub{}
		t.Cleanup(func() { c.Transport = previous })
	}
	rec := httptest.NewRecorder()
	authzhandler.GetPermissions(rec, withSession(httptest.NewRequest(http.MethodGet, "/", nil)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (body %s)", rec.Code, rec.Body.String())
	}
	var resp authzhandler.PermissionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Roles) != constants.DefaultIncrementValue {
		t.Fatalf("roles = %+v, want exactly one", resp.Roles)
	}
	if only := resp.Roles[constants.DefaultInitValue]; only.RoleID != "r-bad" || !only.IsExpired {
		t.Fatalf("roles = %+v, want only r-bad marked expired", resp.Roles)
	}
}

const (
	sourceGroupID   = "ug-ops"
	sourceGroupName = "Operations"
	sourceRoleID    = "r-ops"
)

// groupStub serves a user who holds one role only through one group, and the
// caller's live session for every other lookup.
type groupStub struct{}

func (groupStub) RoundTrip(r *http.Request) (*http.Response, error) {
	switch {
	case strings.Contains(r.URL.Path, "/groups/"+sourceGroupID):
		return envelope(http.StatusOK, groupdata.Group{ID: sourceGroupID, Name: sourceGroupName, RoleRefs: []string{sourceRoleID}})
	case strings.Contains(r.URL.Path, "/accessroles/"+sourceRoleID):
		return envelope(http.StatusOK, roledata.AccessRole{ID: sourceRoleID, Status: roledata.RoleStatusActive})
	case strings.Contains(r.URL.Path, "/users/"):
		group := sourceGroupID
		return envelope(http.StatusOK, userresource.User{ID: "uid", GroupRefs: []*string{&group}})
	default:
		return liveSession()
	}
}

// An inherited role names its group as well as its id, from the group the
// resolution already read, so the UI needs no lookup of its own.
func TestGetPermissionsNamesTheSourceGroup(t *testing.T) {
	stubExporter(t, groupStub{})
	for _, c := range []*http.Client{clients.GetGroupClient().GetHTTPClient(), clients.GetAccessRoleClient().GetHTTPClient()} {
		previous := c.Transport
		c.Transport = groupStub{}
		t.Cleanup(func() { c.Transport = previous })
	}
	rec := httptest.NewRecorder()
	authzhandler.GetPermissions(rec, withSession(httptest.NewRequest(http.MethodGet, "/", nil)))
	testutil.Equal(t, "status", rec.Code, http.StatusOK)
	testutil.Equal(t, "wire name", strings.Contains(rec.Body.String(), `"groupName":"`+sourceGroupName+`"`), true)
	var resp authzhandler.PermissionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := []authzhandler.RoleSource{{Kind: constants.RoleSourceInherited, GroupID: sourceGroupID, GroupName: sourceGroupName}}
	if len(resp.Roles) != constants.DefaultIncrementValue || !slices.Equal(resp.Roles[constants.DefaultInitValue].Sources, want) {
		t.Fatalf("roles = %+v, want one role inherited from %s", resp.Roles, sourceGroupName)
	}
}
