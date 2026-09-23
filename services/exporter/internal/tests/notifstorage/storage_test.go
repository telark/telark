package notifstorage

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/telark/exporter/internal/constants"
	exprdb "github.com/telark/exporter/internal/redis"
	notifstorage "github.com/telark/exporter/internal/redis/notifications"
	notiftypes "github.com/telark/exporter/internal/types/notifications"
)

const (
	testUserID = "u1"

	listLimit      = 10
	oversizedLimit = 100000

	seededNotifications = 3
)

func newStorage(t *testing.T) *notifstorage.Storage {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	exprdb.Set(client)
	s, err := notifstorage.NewStorage()
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	return s
}

func notif() notiftypes.Notification {
	return notiftypes.Notification{
		UserID:   testUserID,
		Type:     notiftypes.TypeRoleChanged,
		Title:    "title",
		Message:  "message",
		Severity: notiftypes.SeverityInfo,
	}
}

func TestNewStorageWithoutClient(t *testing.T) {
	exprdb.Set(nil)
	if _, err := notifstorage.NewStorage(); err == nil {
		t.Error("NewStorage should fail without a redis client")
	}
}

func TestEmitAndList(t *testing.T) {
	ctx := context.Background()
	s := newStorage(t)

	n := notif()
	n.Metadata = map[string]any{notiftypes.MetaKeyTargetID: "tgt", "extra": "v"}
	created, err := s.Emit(ctx, n)
	if err != nil || created.ID == constants.EmptyString {
		t.Fatalf("Emit = %+v, err %v", created, err)
	}

	list, err := s.List(ctx, testUserID, listLimit, constants.EmptyString)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list.Items) != constants.DefaultIncrementValue || list.UnreadCount != constants.DefaultIncrementValue {
		t.Fatalf("List items=%d unread=%d, want 1/1", len(list.Items), list.UnreadCount)
	}
	if list.Items[constants.DefaultInitValue].Title != "title" || list.Items[constants.DefaultInitValue].Metadata["extra"] != "v" {
		t.Errorf("round-tripped notification wrong: %+v", list.Items[constants.DefaultInitValue])
	}
}

func TestEmitDedupUpdatesInPlace(t *testing.T) {
	ctx := context.Background()
	s := newStorage(t)

	n := notif()
	n.Metadata = map[string]any{notiftypes.MetaKeyTargetID: "same-target"}
	first, err := s.Emit(ctx, n)
	if err != nil {
		t.Fatal(err)
	}

	n.Title = "updated title"
	second, err := s.Emit(ctx, n)
	if err != nil {
		t.Fatal(err)
	}
	// Same target within the dedup window updates the existing notification.
	if second.ID != first.ID {
		t.Errorf("dedup created a new notification: %s vs %s", second.ID, first.ID)
	}

	list, _ := s.List(ctx, testUserID, listLimit, constants.EmptyString)
	if len(list.Items) != constants.DefaultIncrementValue {
		t.Fatalf("dedup left %d items, want 1", len(list.Items))
	}
	if list.Items[constants.DefaultInitValue].Title != "updated title" {
		t.Errorf("dedup did not update title: %q", list.Items[constants.DefaultInitValue].Title)
	}
}

func TestMarkRead(t *testing.T) {
	ctx := context.Background()
	s := newStorage(t)
	created, err := s.Emit(ctx, notif())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.MarkRead(ctx, testUserID, created.ID); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	list, _ := s.List(ctx, testUserID, listLimit, constants.EmptyString)
	if list.UnreadCount != constants.DefaultInitValue {
		t.Errorf("unread = %d after MarkRead, want 0", list.UnreadCount)
	}
	if len(list.Items) == constants.DefaultIncrementValue && list.Items[constants.DefaultInitValue].ReadAt == nil {
		t.Error("read notification has no ReadAt timestamp")
	}
}

func TestMarkAllReadAndClear(t *testing.T) {
	ctx := context.Background()
	s := newStorage(t)
	for range seededNotifications {
		if _, err := s.Emit(ctx, notif()); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.MarkAllRead(ctx, testUserID); err != nil {
		t.Fatalf("MarkAllRead: %v", err)
	}
	list, _ := s.List(ctx, testUserID, listLimit, constants.EmptyString)
	if list.UnreadCount != constants.DefaultInitValue {
		t.Errorf("unread = %d after MarkAllRead, want 0", list.UnreadCount)
	}

	if err := s.Clear(ctx, testUserID); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	cleared, _ := s.List(ctx, testUserID, listLimit, constants.EmptyString)
	if len(cleared.Items) != constants.DefaultInitValue {
		t.Errorf("Clear left %d items", len(cleared.Items))
	}
}

func TestListLimitBounds(t *testing.T) {
	ctx := context.Background()
	s := newStorage(t)
	if _, err := s.Emit(ctx, notif()); err != nil {
		t.Fatal(err)
	}
	// A non-positive limit falls back to the default; an oversized one is capped.
	if _, err := s.List(ctx, testUserID, constants.DefaultInitValue, constants.EmptyString); err != nil {
		t.Errorf("default-limit list failed: %v", err)
	}
	if _, err := s.List(ctx, testUserID, oversizedLimit, constants.EmptyString); err != nil {
		t.Errorf("capped-limit list failed: %v", err)
	}
}
