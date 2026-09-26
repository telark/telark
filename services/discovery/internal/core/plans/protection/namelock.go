package protection

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection/validation"
	xwareredis "github.com/telark/x-ware/redis/stream"
)

// Sentinels for the name-lock outcomes so the handler can map them with errors.Is.
var (
	ErrNameInFlight            = errors.New(string(ErrPlanNameInFlight))
	ErrCoordinationUnavailable = errors.New(string(ErrPlanCoordinationUnavailable))
)

func NewNameLocks(client *xwareredis.LockClient) *NameLocks {
	return &NameLocks{client: client}
}

// Holds the normalized name from the uniqueness check through the write that follows it, so
// two parallel creates or renames cannot both pass UniqueName.
func (l *NameLocks) Acquire(ctx context.Context, name string) (func(), error) {
	key := validation.NormalizeName(name)
	if l.client == nil {
		return l.acquireLocal(key)
	}
	redisKey := KeyPrefixLockPlanName + key
	value := uuid.NewString()
	acquired, err := l.client.Acquire(ctx, redisKey, value, constants.PlanLockTTL)
	if err != nil {
		return nil, fmt.Errorf(fmtWrappedErr, ErrCoordinationUnavailable, err)
	}
	if !acquired {
		return nil, ErrNameInFlight
	}
	return HoldLock(l.client, redisKey, value), nil
}

// Exporter calls ignore the request deadline, so a holder can outrun any fixed TTL; the heartbeat
// keeps the lock while the holder lives, and the TTL only frees it after a crash.
func HoldLock(client *xwareredis.LockClient, key, value string) func() {
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(constants.PlanLockHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				_, _ = client.Extend(context.Background(), key, value, constants.PlanLockTTL)
			}
		}
	}()
	return func() {
		close(stop)
		<-done
		_ = client.Release(context.Background(), key, value)
	}
}

// Without Redis (standalone bootstrap) a per-name mutex keeps two writers of one replica apart.
func (l *NameLocks) acquireLocal(key string) (func(), error) {
	entry, _ := l.local.LoadOrStore(key, &sync.Mutex{})
	mu, ok := entry.(*sync.Mutex)
	if !ok || !mu.TryLock() {
		return nil, ErrNameInFlight
	}
	return mu.Unlock, nil
}
