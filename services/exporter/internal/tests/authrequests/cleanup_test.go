package authrequests

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/telark/telark/internal/data/resources/authrequest"
	"github.com/telark/telark/services/exporter/internal/authrequests"
	"github.com/telark/telark/services/exporter/internal/constants"
	envmanager "github.com/telark/telark/services/exporter/internal/managers/envs"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

const (
	longAgoUnix   = 900_000_000
	oldName       = "req-old"
	lateName      = "req-old-late"
	atMarkName    = "req-at-the-mark"
	nearMarkName  = "req-near-the-mark"
	freshName     = "req-fresh"
	goneName      = "req-gone"
	stuckName     = "req-stuck-a"
	alsoStuckName = "req-stuck-b"
	twoDaysName   = "req-two-days"
	twoDays       = 2 * day
	oneDayValue   = "1"
)

// A bubble's clock starts in 2000, so this is past any retention.
var longAgo = time.Unix(longAgoUnix, constants.DefaultInitValue).UTC()

func firstSweep(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	go authrequests.StartCleanup(ctx)
	synctest.Wait()
	cancel()
	synctest.Wait()
}

// The start sweeps once and each interval sweeps again with its own time: a request a second inside the retention
// stays at the start and goes an hour later, whatever its phase.
func TestCleanupDeletesOnlyOldRequests(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		now := time.Now()
		client := installFake(t,
			request(oldName, authrequest.PhaseCompleted, longAgo),
			request(atMarkName, authrequest.PhasePending, now.Add(-defaultRetention)),
			request(nearMarkName, authrequest.PhaseCompleted, now.Add(time.Second-defaultRetention)),
			request(freshName, authrequest.PhaseCompleted, now),
		)
		ctx, cancel := context.WithCancel(t.Context())
		stopped := make(chan struct{})
		go func() {
			authrequests.StartCleanup(ctx)
			close(stopped)
		}()
		synctest.Wait()
		equal(t, "after the start", storedNames(t, client), freshName+nameSep+nearMarkName)

		late := request(lateName, authrequest.PhaseFailed, longAgo)
		if _, err := client.Resource(gvr).Namespace(md.Namespace).Create(ctx, late, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(constants.AuthRequestCleanupInterval)
		synctest.Wait()
		equal(t, "after an interval", storedNames(t, client), freshName)

		cancel()
		<-stopped
	})
}

func TestCleanupReadsTheRetention(t *testing.T) {
	t.Cleanup(func() { _ = envmanager.InitAuthRequestRetention() })
	for value, kept := range map[string]string{constants.EmptyString: twoDaysName, oneDayValue: constants.EmptyString} {
		t.Run("days "+value, func(t *testing.T) {
			t.Setenv(constants.AuthRequestRetentionDaysEnv, value)
			if err := envmanager.InitAuthRequestRetention(); err != nil {
				t.Fatal(err)
			}
			synctest.Test(t, func(t *testing.T) {
				client := installFake(t, request(twoDaysName, authrequest.PhaseCompleted, time.Now().Add(-twoDays)))
				firstSweep(t)
				equal(t, "kept", storedNames(t, client), kept)
			})
		})
	}
}

// A request another replica deleted first counts as deleted, a refused delete is skipped and the sweep goes on to the
// rest, and a failed list deletes nothing.
func TestCleanupSurvivesFailures(t *testing.T) {
	cases := []struct {
		name    string
		listErr error
		deletes int
		kept    string
	}{
		{"deletes that fail", nil, len(doomed()), stuckName + nameSep + alsoStuckName},
		{"a failed list", errDown, none, goneName + nameSep + oldName + nameSep + stuckName + nameSep + alsoStuckName},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				client := installFake(t)
				for _, name := range doomed() {
					if err := client.Tracker().Create(gvr, request(name, authrequest.PhaseCompleted, longAgo), md.Namespace); err != nil {
						t.Fatal(err)
					}
				}
				client.PrependReactor(verbDelete, md.Plural, failingDeletes(client))
				if c.listErr != nil {
					answer(client, verbList, nil, c.listErr)
				}

				firstSweep(t)

				equal(t, "delete calls", countOf(client, verbDelete), c.deletes)
				equal(t, "kept", storedNames(t, client), c.kept)
			})
		})
	}
}

func doomed() []string {
	return []string{stuckName, goneName, alsoStuckName, oldName}
}

// The gone request leaves the store before its delete lands, as when another replica's sweep won.
func failingDeletes(client *dynamicfake.FakeDynamicClient) k8stesting.ReactionFunc {
	return func(action k8stesting.Action) (bool, runtime.Object, error) {
		deleted, isDelete := action.(k8stesting.DeleteAction)
		if !isDelete {
			return false, nil, nil
		}
		switch deleted.GetName() {
		case stuckName, alsoStuckName:
			return true, nil, errDown
		case goneName:
			if err := client.Tracker().Delete(gvr, md.Namespace, goneName); err != nil {
				return true, nil, err
			}
			return true, nil, apierrors.NewNotFound(gvr.GroupResource(), goneName)
		}
		return false, nil, nil
	}
}
