package insightsindex_test

import (
	"context"
	stderrors "errors"
	"slices"
	"sync"
	"testing"
	"time"

	insightsdata "github.com/telark/telark/internal/data/insights"
	"github.com/telark/telark/internal/data/resources/application"
	roledata "github.com/telark/telark/internal/data/resources/role"
	userresource "github.com/telark/telark/internal/data/resources/user"
	notifclient "github.com/telark/telark/internal/rest/clients/notifications"
	"github.com/telark/telark/internal/x-ware/async"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/insightsindex"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	readerOne   = "user-reader-1"
	readerTwo   = "user-reader-2"
	outsider    = "user-applications-only"
	grantsDown  = "user-grants-unavailable"
	nsExcluded  = "kube-system"
	scoreFourth = 4000
	scoreFifth  = 5000
	noneSent    = 0
	wantMessage = "**" + idC1 + "** in **" + nsShop + "/" + appWeb + "**."
)

var (
	errUsersDown    = stderrors.New("users list unavailable")
	errExcludedDown = stderrors.New("excluded namespaces unavailable")
	errGrantsDown   = stderrors.New("grants unavailable")
	errEmitFailed   = stderrors.New("notification refused")
)

type emitRecorder struct {
	mu      sync.Mutex
	sent    []notifclient.Notification
	failFor string
}

func (r *emitRecorder) emit(_ context.Context, n notifclient.Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n.UserID == r.failFor {
		return errEmitFailed
	}
	r.sent = append(r.sent, n)
	return nil
}

// Waits for the dispatched fan-out, then hands over and forgets what was sent.
func (r *emitRecorder) take() []notifclient.Notification {
	async.Drain()
	r.mu.Lock()
	defer r.mu.Unlock()
	sent := r.sent
	r.sent = nil
	return sent
}

func grantsFor(userID string) (xauthz.Grants, error) {
	if userID == grantsDown {
		return xauthz.Grants{}, errGrantsDown
	}
	scope := roledata.ScopeInsights
	if userID == outsider {
		scope = roledata.ScopeApplications
	}
	return xauthz.Grants{Levels: map[string]roledata.PermissionLevel{scope: roledata.PermissionLevelOwner}}, nil
}

type notifyFixture struct {
	*fixture
	rec         *emitRecorder
	usersErr    error
	excludedErr error
}

func (f *notifyFixture) notifier() *insightsindex.CriticalNotifier {
	return &insightsindex.CriticalNotifier{
		ListUsers: func() ([]*userresource.User, error) {
			if f.usersErr != nil {
				return nil, f.usersErr
			}
			return []*userresource.User{{ID: readerOne}, {ID: outsider}, {ID: grantsDown}, {ID: readerTwo}}, nil
		},
		Grants: grantsFor,
		Excluded: func(context.Context) ([]string, error) {
			if f.excludedErr != nil {
				return nil, f.excludedErr
			}
			return []string{nsExcluded}, nil
		},
		Emit: f.rec.emit,
	}
}

// A replica that has not loaded yet: a new process, or a second replica, on the same Redis.
func (f *notifyFixture) replica() *insightsindex.Index {
	return insightsindex.New(insightsindex.Settings{
		Refresh: time.Second, Resync: time.Second, StaleAfter: staleAfter, Notifier: f.notifier(),
	})
}

// Past the first load ever, which latches whatever is already open without notifying.
func newNotifyFixture(t *testing.T) *notifyFixture {
	t.Helper()
	async.Init()
	f := &notifyFixture{fixture: newFixture(t), rec: &emitRecorder{}}
	f.idx = f.replica()
	f.refresh(t)
	return f
}

func incident(severity, status string) application.Insight {
	c := card(idC1, severity, time.Hour)
	c.Status = status
	return c
}

func recipients(sent []notifclient.Notification) []string {
	out := make([]string, firstItem, len(sent))
	for i := range sent {
		out = append(out, sent[i].UserID)
	}
	slices.Sort(out)
	return out
}

func assertNotified(t *testing.T, name string, sent []notifclient.Notification) {
	t.Helper()
	assertIDs(t, name, recipients(sent), readerOne, readerTwo)
}

func assertSilent(t *testing.T, name string, sent []notifclient.Notification) {
	t.Helper()
	testutil.Equal(t, name, len(sent), noneSent)
}

func TestNewCriticalIncidentNotifiesEachInsightsReaderOnce(t *testing.T) {
	f := newNotifyFixture(t)
	f.put(t, nsShop, appWeb, scoreFirst, incident(application.InsightSeverityCritical, application.InsightStatusOpen))
	f.refresh(t)

	sent := f.rec.take()
	assertNotified(t, "readers of insights only", sent)
	for i := range sent {
		testutil.Equal(t, "type", sent[i].Type, notifclient.TypeIncidentCritical)
		testutil.Equal(t, "severity", sent[i].Severity, notifclient.SeverityError)
		testutil.Equal(t, "title", sent[i].Title, string(constants.NotifIncidentCriticalTitle))
		testutil.Equal(t, "message", sent[i].Message, wantMessage)
		testutil.Equal(t, "deep link id", sent[i].Metadata[notifclient.MetaKeyInsightID], any(idC1))
		testutil.Equal(t, "dedup target", sent[i].Metadata[notifclient.MetaKeyTargetID], any(idC1))
	}
}

// One episode notifies once, when the card first turns Critical: never for Warning or Info, never
// for an update, and not again after it drops to Warning and back.
func TestOnlyTheFirstCriticalStateOfAnEpisodeNotifies(t *testing.T) {
	f := newNotifyFixture(t)
	steps := []struct {
		name, severity, status string
		notifies               bool
	}{
		{"opens as info", application.InsightSeverityInfo, application.InsightStatusOpen, false},
		{"updated to warning", application.InsightSeverityWarning, application.InsightStatusUpdated, false},
		{"raised to critical", application.InsightSeverityCritical, application.InsightStatusUpdated, true},
		{"updated while critical", application.InsightSeverityCritical, application.InsightStatusUpdated, false},
		{"lowered to warning", application.InsightSeverityWarning, application.InsightStatusUpdated, false},
		{"critical again", application.InsightSeverityCritical, application.InsightStatusUpdated, false},
	}
	for i, step := range steps {
		c := incident(step.severity, step.status)
		c.Title = step.name
		f.put(t, nsShop, appWeb, float64(scoreFirst*(i+constants.DefaultAddValue)), c)
		f.refresh(t)
		if step.notifies {
			assertNotified(t, step.name, f.rec.take())
		} else {
			assertSilent(t, step.name, f.rec.take())
		}
	}
}

// The analyzer keeps one card id per workload, so a recurrence after recovery must notify again.
func TestReopenedOrReturningCriticalIncidentNotifiesAgain(t *testing.T) {
	f := newNotifyFixture(t)
	critical := incident(application.InsightSeverityCritical, application.InsightStatusOpen)
	f.put(t, nsShop, appWeb, scoreFirst, critical)
	f.refresh(t)
	assertNotified(t, "opened", f.rec.take())

	f.put(t, nsShop, appWeb, scoreSecond, incident(application.InsightSeverityCritical, application.InsightStatusResolved))
	f.refresh(t)
	assertSilent(t, "resolved", f.rec.take())

	f.put(t, nsShop, appWeb, scoreThird, critical)
	f.refresh(t)
	assertNotified(t, "reopened", f.rec.take())

	f.mr.Del(insightsdata.DocumentKey(nsShop, appWeb))
	f.resync(t)
	assertSilent(t, "document gone", f.rec.take())

	f.put(t, nsShop, appWeb, scoreFourth, critical)
	f.refresh(t)
	assertNotified(t, "card back", f.rec.take())
}

func TestRecommendationsAndExcludedNamespacesNeverNotify(t *testing.T) {
	f := newNotifyFixture(t)
	recommendation := incident(application.InsightSeverityCritical, application.InsightStatusOpen)
	recommendation.Category = application.InsightCategoryRecommendation
	f.put(t, nsShop, appWeb, scoreFirst, recommendation)

	excludedApp := incident(application.InsightSeverityCritical, application.InsightStatusOpen)
	excludedApp.ID = idC2
	f.put(t, nsExcluded, appAPI, scoreFirst, excludedApp)

	excludedWorkload := incident(application.InsightSeverityCritical, application.InsightStatusOpen)
	excludedWorkload.ID = idC3
	excludedWorkload.Params = map[string]string{cardParamNamespace: nsExcluded}
	f.put(t, nsPay, appLedger, scoreFirst, excludedWorkload)

	f.refresh(t)
	assertSilent(t, "recommendation and excluded namespaces", f.rec.take())
}

// The first load ever stays silent; later loads (a restart) notify what opened while nobody read.
func TestFirstLoadEverIsSilentAndRestartsCatchUp(t *testing.T) {
	async.Init()
	f := &notifyFixture{fixture: newFixture(t), rec: &emitRecorder{}}
	f.idx = f.replica()
	f.put(t, nsShop, appWeb, scoreFirst, incident(application.InsightSeverityCritical, application.InsightStatusOpen))
	f.refresh(t)
	assertSilent(t, "open before the first load ever", f.rec.take())

	restarted := incident(application.InsightSeverityCritical, application.InsightStatusOpen)
	restarted.ID = idC2
	f.put(t, nsPay, appAPI, scoreSecond, restarted)
	f.idx = f.replica()
	f.refresh(t)
	sent := f.rec.take()
	assertNotified(t, "opened while restarting", sent)
	testutil.Equal(t, "only the new incident", sent[firstItem].Metadata[notifclient.MetaKeyInsightID], any(idC2))
}

func TestReplicasNotifyOnce(t *testing.T) {
	f := newNotifyFixture(t)
	other := f.replica()
	if err := other.Refresh(context.Background(), f.rdb); err != nil {
		t.Fatal(err)
	}
	f.put(t, nsShop, appWeb, scoreFirst, incident(application.InsightSeverityCritical, application.InsightStatusOpen))
	f.refresh(t)
	if err := other.Refresh(context.Background(), f.rdb); err != nil {
		t.Fatal(err)
	}
	assertNotified(t, "two replicas, one notification each", f.rec.take())
}

// Unknown recipients release the latch, so the card's next read notifies instead of nobody.
func TestUnknownRecipientsLeaveTheIncidentForALaterRead(t *testing.T) {
	cases := []struct {
		name               string
		usersErr, excluded error
	}{
		{"users unavailable", errUsersDown, nil},
		{"excluded namespaces unavailable", nil, errExcludedDown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newNotifyFixture(t)
			f.usersErr, f.excludedErr = tc.usersErr, tc.excluded
			critical := incident(application.InsightSeverityCritical, application.InsightStatusOpen)
			f.put(t, nsShop, appWeb, scoreFirst, critical)
			f.refresh(t)
			assertSilent(t, tc.name, f.rec.take())

			f.usersErr, f.excludedErr = nil, nil
			f.put(t, nsShop, appWeb, scoreFifth, critical)
			f.refresh(t)
			assertNotified(t, "next read", f.rec.take())
		})
	}
}

func TestAFailedNotificationStillReachesTheOtherReaders(t *testing.T) {
	f := newNotifyFixture(t)
	f.rec.failFor = readerOne
	f.put(t, nsShop, appWeb, scoreFirst, incident(application.InsightSeverityCritical, application.InsightStatusOpen))
	f.refresh(t)
	assertIDs(t, "the other reader", recipients(f.rec.take()), readerTwo)
}

// A latch Redis refuses claims nothing, so nobody is notified until a read can latch the card.
func TestALatchRedisRefusesNotifiesNobodyUntilItClears(t *testing.T) {
	f := newNotifyFixture(t)
	latch := constants.KeyPrefixInsightsNotified + idC1
	if _, err := f.mr.Lpush(latch, idC1); err != nil {
		t.Fatal(err)
	}
	critical := incident(application.InsightSeverityCritical, application.InsightStatusOpen)
	f.put(t, nsShop, appWeb, scoreFirst, critical)
	f.refresh(t)
	assertSilent(t, "latch of the wrong type", f.rec.take())

	f.mr.Del(latch)
	f.put(t, nsShop, appWeb, scoreFifth, critical)
	f.refresh(t)
	assertNotified(t, "latch cleared", f.rec.take())
}
