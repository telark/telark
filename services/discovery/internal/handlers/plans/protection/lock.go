package protection

import (
	"context"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection"
)

var localPlanDecisionLocks sync.Map

// A per-request value keeps a TTL-expired holder from releasing the next
// owner's lock; Release only deletes when the stored value matches.
func lockPlanDecision(ctx context.Context, w http.ResponseWriter, planID string) (func(), bool) {
	coord := getCoordinationBundle()
	if coord == nil {
		return lockPlanDecisionLocal(w, planID)
	}
	key := constants.KeyPrefixLockPlanDecision + planID
	value := uuid.NewString()
	acquired, err := coord.Lock.Acquire(ctx, key, value, constants.PlanLockTTL)
	if err != nil {
		respondError(w, http.StatusServiceUnavailable, protection.ErrPlanCoordinationUnavailable, err)
		return nil, false
	}
	if !acquired {
		respondError(w, http.StatusConflict, protection.ErrPlanDecisionInFlight, nil)
		return nil, false
	}
	return protection.HoldLock(coord.Lock, key, value), true
}

// Without a bundle (standalone bootstrap, or consumer-group setup failed) the
// routes still serve, so a per-plan mutex is what keeps two deciders apart.
func lockPlanDecisionLocal(w http.ResponseWriter, planID string) (func(), bool) {
	entry, _ := localPlanDecisionLocks.LoadOrStore(planID, &sync.Mutex{})
	mu, ok := entry.(*sync.Mutex)
	if !ok || !mu.TryLock() {
		respondError(w, http.StatusConflict, protection.ErrPlanDecisionInFlight, nil)
		return nil, false
	}
	return mu.Unlock, true
}
