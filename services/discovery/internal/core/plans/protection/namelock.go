package protection

import (
	"context"
	"errors"
	"fmt"
	"sync"

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
// two parallel creates or renames cannot both pass UniqueName. The TTL outlives the deploy budget.
func (l *NameLocks) Acquire(ctx context.Context, name string) (func(), error) {
	key := validation.NormalizeName(name)
	if l.client == nil {
		return l.acquireLocal(key)
	}
	redisKey := KeyPrefixLockPlanName + key
	value := uuid.NewString()
	acquired, err := l.client.Acquire(ctx, redisKey, value, constants.ProtectionPlanDeployTimeout)
	if err != nil {
		return nil, fmt.Errorf(fmtWrappedErr, ErrCoordinationUnavailable, err)
	}
	if !acquired {
		return nil, ErrNameInFlight
	}
	return func() { _ = l.client.Release(context.Background(), redisKey, value) }, nil
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
