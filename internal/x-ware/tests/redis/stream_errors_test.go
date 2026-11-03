package redis

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/telark/telark/internal/x-ware/constants"
	"github.com/telark/telark/internal/x-ware/redis/stream"
)

const (
	leaderKey    = "leader"
	leaderID     = "replica-a"
	lockKey      = "lock:reconcile"
	valueKey     = "settings"
	graceApp     = "checkout"
	gracePrefix  = "grace:scale:"
	listItem     = "item"
	readerGroup  = "workers"
	missingGroup = "absent"
	cmdGet       = "get"
	electionTTL  = time.Minute
)

type keyRead struct {
	name string
	key  string
	read func(ctx context.Context, rdb *goredis.Client) (empty bool, err error)
}

// Rewrites the watched key after every GET, so the transaction that follows aborts.
type conflictHook struct {
	t  *testing.T
	mr *miniredis.Miniredis
}

func (conflictHook) DialHook(next goredis.DialHook) goredis.DialHook { return next }

func (conflictHook) ProcessPipelineHook(next goredis.ProcessPipelineHook) goredis.ProcessPipelineHook {
	return next
}

func (h conflictHook) ProcessHook(next goredis.ProcessHook) goredis.ProcessHook {
	return func(ctx context.Context, cmd goredis.Cmder) error {
		err := next(ctx, cmd)
		if strings.EqualFold(cmd.Name(), cmdGet) {
			if setErr := h.mr.Set(leaderKey, leaderID); setErr != nil {
				h.t.Error(setErr)
			}
		}
		return err
	}
}

func miniredisClient(t *testing.T) (*miniredis.Miniredis, *goredis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

func election(rdb *goredis.Client) *stream.ElectionClient {
	return stream.NewElectionClient(rdb, leaderID, leaderKey, electionTTL)
}

// A missing key reads as empty; any other Redis error (here WRONGTYPE) is returned.
func assertMissingIsEmpty(t *testing.T, cases []keyRead) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mr, rdb := miniredisClient(t)
			if empty, err := tc.read(t.Context(), rdb); err != nil || !empty {
				t.Fatalf("missing key: empty = %v, err = %v, want empty and no error", empty, err)
			}
			if _, err := mr.Lpush(tc.key, listItem); err != nil {
				t.Fatal(err)
			}
			if _, err := tc.read(t.Context(), rdb); err == nil {
				t.Error("wrong-type key: want an error")
			}
		})
	}
}

func TestElectionClient_MissingKey(t *testing.T) {
	assertMissingIsEmpty(t, []keyRead{
		{name: "is leader", key: leaderKey, read: func(ctx context.Context, rdb *goredis.Client) (bool, error) {
			held, err := election(rdb).IsLeader(ctx)
			return !held, err
		}},
		{name: "current leader", key: leaderKey, read: func(ctx context.Context, rdb *goredis.Client) (bool, error) {
			id, err := election(rdb).CurrentLeader(ctx)
			return id == constants.EmptyString, err
		}},
		{name: "resign", key: leaderKey, read: func(ctx context.Context, rdb *goredis.Client) (bool, error) {
			return true, election(rdb).Resign(ctx)
		}},
		{name: "clear if held", key: leaderKey, read: func(ctx context.Context, rdb *goredis.Client) (bool, error) {
			cleared, err := election(rdb).ClearIfHeld(ctx, leaderID)
			return !cleared, err
		}},
		{name: "renew", key: leaderKey, read: func(ctx context.Context, rdb *goredis.Client) (bool, error) {
			renewed, err := election(rdb).Renew(ctx)
			return !renewed, err
		}},
	})
}

func TestStateLockGraceClients_MissingKey(t *testing.T) {
	assertMissingIsEmpty(t, []keyRead{
		{name: "lock release", key: lockKey, read: func(ctx context.Context, rdb *goredis.Client) (bool, error) {
			return true, stream.NewLockClient(rdb).Release(ctx, lockKey, leaderID)
		}},
		{name: "scale grace", key: gracePrefix + graceApp, read: func(ctx context.Context, rdb *goredis.Client) (bool, error) {
			grace, err := stream.NewGraceClient(rdb).GetScaleGrace(ctx, graceApp)
			return grace == nil, err
		}},
		{name: "operation state", key: statePrefix + opID, read: func(ctx context.Context, rdb *goredis.Client) (bool, error) {
			state, err := stream.NewStateClient(rdb).Get(ctx, opID)
			return state == nil, err
		}},
		{name: "state value", key: valueKey, read: func(ctx context.Context, rdb *goredis.Client) (bool, error) {
			value, err := stream.NewStateClient(rdb).GetValue(ctx, valueKey)
			return value == constants.EmptyString, err
		}},
	})
}

// A WATCH conflict means another replica touched the key: no error, and the key stays.
func TestElectionClient_WatchConflict(t *testing.T) {
	cases := []struct {
		name string
		run  func(ctx context.Context, c *stream.ElectionClient) (bool, error)
	}{
		{name: "resign", run: func(ctx context.Context, c *stream.ElectionClient) (bool, error) {
			return false, c.Resign(ctx)
		}},
		{name: "clear if held", run: func(ctx context.Context, c *stream.ElectionClient) (bool, error) {
			return c.ClearIfHeld(ctx, leaderID)
		}},
		{name: "renew", run: func(ctx context.Context, c *stream.ElectionClient) (bool, error) {
			return c.Renew(ctx)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mr, rdb := miniredisClient(t)
			if err := mr.Set(leaderKey, leaderID); err != nil {
				t.Fatal(err)
			}
			rdb.AddHook(conflictHook{t: t, mr: mr})

			done, err := tc.run(t.Context(), election(rdb))
			if done || err != nil {
				t.Errorf("done = %v, err = %v, want false and no error", done, err)
			}
			if held, _ := mr.Get(leaderKey); held != leaderID {
				t.Errorf("leader = %q, want %q kept", held, leaderID)
			}
		})
	}
}

func TestStreamClient_ReadsWithoutEntries(t *testing.T) {
	_, rdb := miniredisClient(t)
	client := stream.NewStreamClient(rdb)
	if err := client.EnsureConsumerGroup(t.Context(), groupStream, readerGroup); err != nil {
		t.Fatal(err)
	}
	consume := func(ctx context.Context, group string) ([]goredis.XMessage, error) {
		return client.Consume(ctx, groupStream, group, groupReader, readCount, noBlock)
	}
	claim := func(ctx context.Context, group string) ([]goredis.XMessage, error) {
		return client.ClaimStale(ctx, groupStream, group, groupReader, stream.StreamStaleClaimMinIdle, readCount)
	}
	cases := []struct {
		name    string
		read    func(ctx context.Context, group string) ([]goredis.XMessage, error)
		group   string
		wantErr bool
	}{
		{name: "consume an empty group", read: consume, group: readerGroup},
		{name: "consume a missing group", read: consume, group: missingGroup, wantErr: true},
		{name: "claim from a missing group", read: claim, group: missingGroup, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msgs, err := tc.read(t.Context(), tc.group)
			if len(msgs) != none || (err != nil) != tc.wantErr {
				t.Errorf("messages = %v, err = %v, wantErr %v", msgs, err, tc.wantErr)
			}
		})
	}
}
