package notifhandler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	notifhandler "github.com/telark/exporter/internal/handlers/notifications"
	exprdb "github.com/telark/exporter/internal/redis"
	notifstorage "github.com/telark/exporter/internal/redis/notifications"
	notiftypes "github.com/telark/exporter/internal/types/notifications"
	xauthz "github.com/telark/x-ware/authz"
)

func setupRedis(t *testing.T) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	exprdb.Set(client)
}

func asSelf(userID, url string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, url, nil)
	return r.WithContext(xauthz.WithIdentity(r.Context(), xauthz.Identity{UserID: userID}))
}

func TestListHandler(t *testing.T) {
	setupRedis(t)
	rec := httptest.NewRecorder()
	notifhandler.List()(rec, asSelf("u1", "/notifications?userId=u1"))
	if rec.Code != http.StatusOK {
		t.Errorf("list code = %d, want 200", rec.Code)
	}

	noUser := httptest.NewRecorder()
	notifhandler.List()(noUser, asSelf("u1", "/notifications"))
	if noUser.Code != http.StatusBadRequest {
		t.Errorf("missing userId code = %d, want 400", noUser.Code)
	}

	forbidden := httptest.NewRecorder()
	notifhandler.List()(forbidden, asSelf("someone-else", "/notifications?userId=u1"))
	if forbidden.Code != http.StatusForbidden {
		t.Errorf("cross-user list code = %d, want 403", forbidden.Code)
	}
}

func TestEmitHandler(t *testing.T) {
	setupRedis(t)
	valid := `{"userId":"u1","type":"role.changed","title":"t","message":"m","severity":"info"}`
	rec := httptest.NewRecorder()
	notifhandler.Emit()(rec, httptest.NewRequest(http.MethodPost, "/notifications", strings.NewReader(valid)))
	if rec.Code != http.StatusOK {
		t.Errorf("emit code = %d, want 200", rec.Code)
	}

	badBody := httptest.NewRecorder()
	notifhandler.Emit()(badBody, httptest.NewRequest(http.MethodPost, "/notifications", strings.NewReader("{not json")))
	if badBody.Code != http.StatusUnprocessableEntity {
		t.Errorf("bad body code = %d, want 422", badBody.Code)
	}

	invalid := `{"userId":"","type":"t","title":"t","message":"m","severity":"info"}`
	invalidRec := httptest.NewRecorder()
	notifhandler.Emit()(invalidRec, httptest.NewRequest(http.MethodPost, "/notifications", strings.NewReader(invalid)))
	if invalidRec.Code != http.StatusBadRequest {
		t.Errorf("invalid notification code = %d, want 400", invalidRec.Code)
	}
}

func TestMarkReadHandler(t *testing.T) {
	setupRedis(t)
	// Seed a real notification so MarkRead has a target.
	storage, err := notifstorage.NewStorage()
	if err != nil {
		t.Fatal(err)
	}
	created, err := storage.Emit(context.Background(), notiftypes.Notification{
		UserID: "u1", Type: "role.changed", Title: "t", Message: "m", Severity: "info",
	})
	if err != nil {
		t.Fatal(err)
	}

	r := asSelf("u1", "/notifications/"+created.ID+"/read?userId=u1")
	r = mux.SetURLVars(r, map[string]string{"id": created.ID})
	rec := httptest.NewRecorder()
	notifhandler.MarkRead()(rec, r)
	if rec.Code != http.StatusOK {
		t.Errorf("markread code = %d, want 200", rec.Code)
	}
}

func TestMarkAllReadAndClearHandlers(t *testing.T) {
	setupRedis(t)

	allRead := httptest.NewRecorder()
	notifhandler.MarkAllRead()(allRead, asSelf("u1", "/notifications/read-all?userId=u1"))
	if allRead.Code != http.StatusOK {
		t.Errorf("markallread code = %d, want 200", allRead.Code)
	}

	clear := httptest.NewRecorder()
	notifhandler.Clear()(clear, asSelf("u1", "/notifications?userId=u1"))
	if clear.Code != http.StatusOK {
		t.Errorf("clear code = %d, want 200", clear.Code)
	}

	// Guard rejects a caller acting on another user's notifications.
	forbidden := httptest.NewRecorder()
	notifhandler.Clear()(forbidden, asSelf("intruder", "/notifications?userId=u1"))
	if forbidden.Code != http.StatusForbidden {
		t.Errorf("cross-user clear code = %d, want 403", forbidden.Code)
	}
}
