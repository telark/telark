package authrequest

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	requestdata "github.com/telark/telark/internal/data/resources/authrequest"
	userresource "github.com/telark/telark/internal/data/resources/user"
	"github.com/telark/telark/services/auth/internal/constants"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	memberUser    = "u-member"
	notBase64     = "not base64!"
	shortKeyBytes = 16
	redisLoading  = "LOADING Redis is loading the dataset in memory"
	blockedRounds = 3
	// A list that failed leaves the request to the next round.
	listRounds        = 2
	failedFirstRead   = 1
	readsPastFirst    = 3
	failedLockedRead  = 2
	readsPastLocked   = 4
	shutdownAfter     = 10 * time.Second
	olderCreation     = "1999-01-01T00:00:00Z"
	newerCreation     = "1999-01-02T00:00:00Z"
	mixedCaseOther    = " Other@Example.com "
	listReadsLabel    = "pending list reads"
	statusWritesLabel = "status writes"
)

// Nothing but BOOTSTRAP_ADMIN may be created or promoted, a key nobody can open would spend the link for nothing, and a
// request found halfway is one a stopped auth left: each ends before the exporter or Redis is reached.
func TestEndsWithoutReachingTheExporter(t *testing.T) {
	readOnly := constants.BuiltInRoleReadOnly
	member := map[string]*userresource.User{memberUser: {ID: memberUser, Email: otherMailbox, RoleRefs: []*string{&readOnly}}}
	other := func(b *bench) requestdata.AuthRequest { return b.request(firstRequest, mixedCaseOther) }
	keyed := func(key string) func(b *bench) requestdata.AuthRequest {
		return func(b *bench) requestdata.AuthRequest {
			request := b.request(firstRequest, adminMailbox)
			request.RecipientKey = key
			return request
		}
	}
	left := func(phase string) func(b *bench) requestdata.AuthRequest {
		return func(b *bench) requestdata.AuthRequest {
			request := b.request(firstRequest, adminMailbox)
			request.Phase = phase
			return request
		}
	}
	denied := ending{[]string{requestdata.PhaseDenied}, fmt.Sprintf(constants.AuthRequestNotBootstrap, otherMailbox)}
	badKey := ending{[]string{requestdata.PhaseFailed}, string(constants.ErrAuthRequestRecipientKey)}
	interrupted := ending{[]string{requestdata.PhaseFailed}, constants.AuthRequestInterrupted}
	cases := []silentCase{
		{"another email on a fresh install", nil, other, denied},
		{"another email that has an account", member, other, denied},
		{"a recipient key that is not base64", nil, keyed(notBase64), badKey},
		{"a recipient key of the wrong length", nil, keyed(base64.StdEncoding.EncodeToString(make([]byte, shortKeyBytes))), badKey},
		{"a request left recovering", nil, left(requestdata.PhaseRecovering), interrupted},
		{"a request left with its link", nil, left(requestdata.PhaseLinkReady), interrupted},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				b := newBench(t, adminMailbox, &testutil.FakeExporter{Users: c.users})
				b.begin(t, c.request(b))
				b.end(t, firstRequest)
				b.expect(t, firstRequest, c.want)
				testutil.Equal(t, othersLabel, b.others.Load(), constants.DefaultInitValue)
				testutil.Equal(t, tokensLabel, enrollTokenOwners(), constants.EmptyString)
				testutil.Equal(t, lockLabel, redisServer.Exists(constants.RedisKeyAuthRequestRun), false)
			})
		})
	}
}

// One run at a time: while another run holds the lock, or Redis can't give it, a round runs nothing, reports nothing
// and leaves the lock alone. The request runs once the lock is free.
func TestABlockedRunLockRunsNothing(t *testing.T) {
	cases := []blockedCase{
		{"another run holds the lock",
			func() { _ = redisServer.Set(constants.RedisKeyAuthRequestRun, takenOver) },
			func() { redisServer.Del(constants.RedisKeyAuthRequestRun) }, takenOver},
		{"Redis cannot answer",
			func() { redisServer.SetError(redisLoading) },
			func() { redisServer.SetError(constants.EmptyString) }, constants.EmptyString},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				b := newBench(t, adminMailbox, waitingExporter())
				c.block()
				t.Cleanup(c.release)
				b.begin(t)
				time.Sleep(blockedRounds*constants.AuthRequestPollInterval + lookEvery)
				testutil.Equal(t, listReadsLabel, b.fake.AuthRequestListReads(), blockedRounds)
				testutil.Equal(t, statusWritesLabel, len(b.fake.StatusWrites()), constants.DefaultInitValue)
				testutil.Equal(t, othersLabel, b.others.Load(), constants.DefaultInitValue)
				held, _ := redisServer.Get(constants.RedisKeyAuthRequestRun)
				testutil.Equal(t, lockLabel, held, c.lock)

				c.release()
				b.end(t, firstRequest)
				testutil.Equal(t, phasesLabel, b.phases(firstRequest), requestdata.PhaseWaitingForApproval)
			})
		})
	}
}

// A pending list the exporter fails, before the lock or under it, runs nothing that round and releases the lock.
func TestAFailedListWaitsForTheNextRound(t *testing.T) {
	cases := []failedListCase{
		{"the first read fails", failedFirstRead, readsPastFirst},
		{"the read under the lock fails", failedLockedRead, readsPastLocked},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				b := newBench(t, adminMailbox, waitingExporter())
				b.fake.FailAuthRequestLists(c.failed)
				began := time.Now()
				b.run(t)
				testutil.Equal(t, "ran a round later", time.Since(began) >= listRounds*constants.AuthRequestPollInterval, true)
				testutil.Equal(t, listReadsLabel, b.fake.AuthRequestListReads(), c.reads)
				testutil.Equal(t, phasesLabel, b.phases(firstRequest), requestdata.PhaseWaitingForApproval)
				testutil.Equal(t, lockLabel, redisServer.Exists(constants.RedisKeyAuthRequestRun), false)
			})
		})
	}
}

// The steps protect the account, not the report: a status the exporter didn't store changes nothing in the run.
func TestAFailedStatusWriteDoesNotStopTheRun(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := newBench(t, adminMailbox, unpinnedExporter(markedAdmin()))
		b.fake.FailNextStatusWrite()
		b.run(t)
		b.expect(t, firstRequest, ending{[]string{requestdata.PhaseExpired}, constants.AuthRequestLinkUnused})
		if b.fake.User(pinnedUser).Status.Invite == nil {
			t.Fatal("the run issued no link after the failed write")
		}
	})
}

// Auth shutting down stops the run where it is: the request ends there with the reason, the next one isn't started,
// the lock is released and a freeze is left to lapse.
func TestStopsWhenAuthShutsDown(t *testing.T) {
	enrollment := func() *testutil.FakeExporter { return unpinnedExporter(markedAdmin()) }
	recovering, ready, failed := requestdata.PhaseRecovering, requestdata.PhaseLinkReady, requestdata.PhaseFailed
	cases := []shutdownCase{
		{"during the hold", signingRecovery, recovering, false, true, []string{recovering, failed}},
		{"while a recovery waits for its passkey", signingRecovery, ready, false, true, []string{recovering, ready, failed}},
		{"during the hold before the last look", signingRecovery, ready, true, true, []string{recovering, ready, failed}},
		{"while an enrollment waits for its passkey", enrollment, ready, false, false, []string{ready, failed}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				b := newBench(t, adminMailbox, c.fake())
				b.begin(t, b.request(firstRequest, adminMailbox), b.request(secondRequest, adminMailbox))
				b.await(t, firstRequest, c.until)
				if c.lapse {
					redisServer.Del(frozenKey)
				}
				time.Sleep(shutdownAfter)
				b.halt()
				b.expect(t, firstRequest, ending{c.phases, constants.AuthRequestInterrupted})
				testutil.Equal(t, "the next request's writes", len(b.writes(secondRequest)), constants.DefaultInitValue)
				testutil.Equal(t, "frozen", redisServer.Exists(frozenKey), c.frozen)
				testutil.Equal(t, lockLabel, redisServer.Exists(constants.RedisKeyAuthRequestRun), false)
			})
		})
	}
}

func TestRunsTheOldestRequestFirst(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := newBench(t, adminMailbox, &testutil.FakeExporter{})
		newer, older := b.request(firstRequest, otherMailbox), b.request(secondRequest, otherMailbox)
		newer.CreatedAt, older.CreatedAt = newerCreation, olderCreation
		b.begin(t, newer, older)
		b.end(t, firstRequest)
		writes := b.fake.StatusWrites()
		order := make([]string, constants.DefaultInitValue, len(writes))
		for _, write := range writes {
			order = append(order, write.ID+constants.ColonSeparator+write.Status.Phase)
		}
		denied := constants.ColonSeparator + requestdata.PhaseDenied
		testutil.Equal(t, statusWritesLabel, strings.Join(order, listSep), secondRequest+denied+listSep+firstRequest+denied)
	})
}
