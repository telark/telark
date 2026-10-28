package notifstorage

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	dataconstants "github.com/telark/telark/internal/data/constants"
	notifclient "github.com/telark/telark/internal/rest/clients/notifications"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
	exprdb "github.com/telark/telark/services/exporter/internal/redis"
	notifstorage "github.com/telark/telark/services/exporter/internal/redis/notifications"
	notiftypes "github.com/telark/telark/services/exporter/internal/types/notifications"
)

const (
	testUserID  = "u1"
	otherUserID = "u2"
	thirdUserID = "u3"
	testTarget  = "tgt"

	listLimit      = 10
	oversizedLimit = 100000

	seededNotifications = 3
	concurrentCallers   = 32
	unreadBeforeDelete  = 2
	unreadAfterDelete   = 1
	distinctEvents      = 2
	testServiceToken    = "svc-token"
	// Past the 10-minute webhook marker, well inside the 7-day dedup of an unread copy.
	markerExpiry = 11 * time.Minute
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

// Seen live: "Added to group" then "Removed from group" 3 s later left only the removal, since the
// second overwrote the unread first by target. Only a re-published event (same content) is merged.
func TestEmitDedupsOnlyRepublishedEvents(t *testing.T) {
	ctx := context.Background()
	s := newStorage(t)

	n := notif()
	n.Metadata = map[string]any{notiftypes.MetaKeyTargetID: "same-target"}
	first, err := s.Emit(ctx, n)
	if err != nil {
		t.Fatal(err)
	}
	republished, err := s.Emit(ctx, n)
	if err != nil {
		t.Fatal(err)
	}
	if republished.ID != first.ID {
		t.Errorf("re-published event created a second notification: %s vs %s", republished.ID, first.ID)
	}

	n.Title = "removed"
	if _, err := s.Emit(ctx, n); err != nil {
		t.Fatal(err)
	}
	list, _ := s.List(ctx, testUserID, listLimit, constants.EmptyString)
	if len(list.Items) != distinctEvents || list.UnreadCount != distinctEvents {
		t.Fatalf("items=%d unread=%d, want 2/2: another change to the same target is its own notification",
			len(list.Items), list.UnreadCount)
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

// Two tabs marking the same notification read at once decremented the unread count twice.
func TestConcurrentMarkReadCountsOnce(t *testing.T) {
	ctx := context.Background()
	s := newStorage(t)
	target, err := s.Emit(ctx, notif())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Emit(ctx, notif()); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range concurrentCallers {
		wg.Go(func() {
			if err := s.MarkRead(ctx, testUserID, target.ID); err != nil {
				t.Errorf("MarkRead: %v", err)
			}
		})
	}
	wg.Wait()
	list, _ := s.List(ctx, testUserID, listLimit, constants.EmptyString)
	if list.UnreadCount != constants.DefaultIncrementValue {
		t.Errorf("unread = %d after concurrent MarkRead, want 1", list.UnreadCount)
	}
}

func TestDelete(t *testing.T) {
	ctx := context.Background()
	s := newStorage(t)
	unread, err := s.Emit(ctx, notif())
	if err != nil {
		t.Fatal(err)
	}
	read, err := s.Emit(ctx, notif())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.MarkRead(ctx, testUserID, read.ID); err != nil {
		t.Fatal(err)
	}
	kept, err := s.Emit(ctx, notif())
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name, userID, id string
		wantErr          error
		wantUnread       int
	}{
		{name: "someone else's", userID: otherUserID, id: unread.ID, wantErr: notifstorage.ErrNotificationNotFound, wantUnread: unreadBeforeDelete},
		{name: "unread", userID: testUserID, id: unread.ID, wantUnread: unreadAfterDelete},
		{name: "already deleted", userID: testUserID, id: unread.ID, wantErr: notifstorage.ErrNotificationNotFound, wantUnread: unreadAfterDelete},
		{name: "read", userID: testUserID, id: read.ID, wantUnread: unreadAfterDelete},
	}
	for _, tc := range cases {
		if err := s.Delete(ctx, tc.userID, tc.id); !errors.Is(err, tc.wantErr) {
			t.Fatalf("%s: Delete err = %v, want %v", tc.name, err, tc.wantErr)
		}
		list, _ := s.List(ctx, testUserID, listLimit, constants.EmptyString)
		if list.UnreadCount != tc.wantUnread {
			t.Errorf("%s: unread = %d, want %d", tc.name, list.UnreadCount, tc.wantUnread)
		}
	}
	list, _ := s.List(ctx, testUserID, listLimit, constants.EmptyString)
	if len(list.Items) != constants.DefaultIncrementValue || list.Items[constants.DefaultInitValue].ID != kept.ID {
		t.Errorf("after deletes: %+v, want only %s", list.Items, kept.ID)
	}
}

// A deduplicated re-emit racing the delete of the notification it updates must leave either
// a whole notification or none, with the unread count matching what the list shows.
func TestConcurrentDedupEmitAndDeleteStayConsistent(t *testing.T) {
	ctx := context.Background()
	s := newStorage(t)
	n := notif()
	n.Metadata = map[string]any{notiftypes.MetaKeyTargetID: testTarget}
	for range concurrentCallers {
		created, err := s.Emit(ctx, n)
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		wg.Go(func() { _, _ = s.Emit(ctx, n) })
		wg.Go(func() { _ = s.Delete(ctx, testUserID, created.ID) })
		wg.Wait()
	}
	list, _ := s.List(ctx, testUserID, oversizedLimit, constants.EmptyString)
	unread := constants.DefaultInitValue
	for _, item := range list.Items {
		if item.ID == constants.EmptyString || item.UserID != testUserID {
			t.Fatalf("partial notification listed: %+v", item)
		}
		if item.ReadAt == nil {
			unread++
		}
	}
	if list.UnreadCount != unread {
		t.Errorf("unread = %d, list shows %d unread", list.UnreadCount, unread)
	}
}

// Every recipient gets their own notification, while webhooks get each event once, and
// account activity (roles, groups, enroll links) never leaves the bell.
func TestWebhookEventQueuedOncePerEvent(t *testing.T) {
	t.Setenv(dataconstants.EnvServiceToken, testServiceToken)
	ctx := context.Background()
	s := newStorage(t)
	approval := approvalEvent()
	for _, userID := range []string{testUserID, otherUserID, thirdUserID} {
		approval.UserID = userID
		if _, err := s.Emit(ctx, approval); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Emit(ctx, notif()); err != nil {
		t.Fatal(err)
	}

	entries, err := exprdb.Get().XRange(ctx, notifclient.EventsStream, "-", "+").Result()
	if err != nil || len(entries) != constants.DefaultIncrementValue {
		t.Fatalf("queued %d events (err %v), want 1", len(entries), err)
	}
	checkQueuedEvent(t, entries[constants.DefaultInitValue].Values, approval)

	approval.Message = "plan p2"
	if _, err := s.Emit(ctx, approval); err != nil {
		t.Fatal(err)
	}
	if n := exprdb.Get().XLen(ctx, notifclient.EventsStream).Val(); n != distinctEvents {
		t.Errorf("after a second event: %d queued, want %d", n, distinctEvents)
	}
}

// The same event again, once the 10-minute marker is gone, merges into the recipient's unread copy
// (the dedup path) and still reaches webhooks: a re-submitted approval is news.
func TestWebhookEventQueuedWhenMergedIntoUnreadCopy(t *testing.T) {
	t.Setenv(dataconstants.EnvServiceToken, testServiceToken)
	ctx := context.Background()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	exprdb.Set(client)
	s, err := notifstorage.NewStorage()
	if err != nil {
		t.Fatal(err)
	}
	approval := approvalEvent()
	approval.UserID = testUserID
	first, err := s.Emit(ctx, approval)
	if err != nil {
		t.Fatal(err)
	}
	mr.FastForward(markerExpiry)
	again, err := s.Emit(ctx, approval)
	if err != nil || again.ID != first.ID {
		t.Fatalf("the repeat did not merge into the unread copy (err %v)", err)
	}
	if n := client.XLen(ctx, notifclient.EventsStream).Val(); n != distinctEvents {
		t.Errorf("%d queued, want %d", n, distinctEvents)
	}
}

// Without the service token nothing can be signed, so nothing is queued, and the notification is still stored.
func TestWebhookEventNeedsTheServiceToken(t *testing.T) {
	t.Setenv(dataconstants.EnvServiceToken, constants.EmptyString)
	ctx := context.Background()
	s := newStorage(t)
	approval := approvalEvent()
	approval.UserID = testUserID
	if _, err := s.Emit(ctx, approval); err != nil {
		t.Fatalf("the notification was refused: %v", err)
	}
	if n := exprdb.Get().XLen(ctx, notifclient.EventsStream).Val(); n != constants.DefaultInitValue {
		t.Errorf("%d unsigned events queued, want none", n)
	}
}

func approvalEvent() notiftypes.Notification {
	return notiftypes.Notification{
		Type:     notifclient.TypePlanApprovalRequested,
		Title:    "Approval requested",
		Message:  "plan p1",
		Severity: notiftypes.SeverityInfo,
		Metadata: map[string]any{notiftypes.MetaKeyTargetID: testTarget},
	}
}

func checkQueuedEvent(t *testing.T, fields map[string]any, want notiftypes.Notification) {
	t.Helper()
	raw, ok := fields[notifclient.EventField].(string)
	payload, verified := xauthz.VerifyRedisEntry(notifclient.EventsStream, raw)
	if !ok || !verified {
		t.Fatalf("the queued entry does not verify: %v", fields)
	}
	var event notifclient.QueuedEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		t.Fatal(err)
	}
	if event.Type != want.Type || event.Title != want.Title || event.Message != want.Message ||
		event.Severity != want.Severity || event.Metadata[notiftypes.MetaKeyTargetID] != testTarget || event.Timestamp <= constants.DefaultInitValue {
		t.Errorf("queued event %+v", event)
	}
	altered := strings.Replace(raw, "plan p1", "plan px", constants.DefaultIncrementValue)
	if _, tampered := xauthz.VerifyRedisEntry(notifclient.EventsStream, altered); tampered {
		t.Error("an altered entry still verifies")
	}
}

// A queue Redis refuses releases the event's marker, so the next copy queues it; the notification itself is stored.
func TestWebhookEventQueueFailureReleasesTheMarker(t *testing.T) {
	t.Setenv(dataconstants.EnvServiceToken, testServiceToken)
	ctx := context.Background()
	s := newStorage(t)
	if err := exprdb.Get().Set(ctx, notifclient.EventsStream, testTarget, redis.KeepTTL).Err(); err != nil {
		t.Fatal(err)
	}
	approval := approvalEvent()
	approval.UserID = testUserID
	if _, err := s.Emit(ctx, approval); err != nil {
		t.Fatalf("the notification was refused: %v", err)
	}

	exprdb.Get().Del(ctx, notifclient.EventsStream)
	approval.UserID = otherUserID
	if _, err := s.Emit(ctx, approval); err != nil {
		t.Fatal(err)
	}
	if n := exprdb.Get().XLen(ctx, notifclient.EventsStream).Val(); n != constants.DefaultIncrementValue {
		t.Errorf("%d queued after the failure, want the event once", n)
	}
}

func TestEmitReturnsAStoreFailure(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	exprdb.Set(client)
	s, err := notifstorage.NewStorage()
	if err != nil {
		t.Fatal(err)
	}
	mr.SetError("LOADING Redis is loading the dataset in memory")
	if _, err := s.Emit(context.Background(), notif()); err == nil {
		t.Fatal("Emit hid a store failure")
	}
}
