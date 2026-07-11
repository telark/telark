package diff

import (
	"strconv"
	"sync"
	"time"

	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/constants"
	"github.com/telark/discovery/core/applications/history/changes"
)

type pendingRemoval struct {
	changes    []application.ApplicationChange
	detectedAt time.Time
}

type stabilizationBuffer struct {
	mu      sync.Mutex
	pending map[string]*pendingRemoval
}

var removalBuffer = &stabilizationBuffer{
	pending: make(map[string]*pendingRemoval),
}

func isRemovalChange(c application.ApplicationChange) bool {
	if c.ChangeType == changes.ChangeTypeRemoved {
		switch c.Field {
		case changes.ChangeFieldImage, changes.ChangeFieldPort,
			changes.ChangeFieldEnvVarKey, changes.ChangeFieldResource,
			changes.ChangeFieldConfigMapRef, changes.ChangeFieldSecretRef,
			changes.ChangeFieldServiceMapping, changes.ChangeFieldIngressRule:
			return true
		}
	}
	if c.Field == changes.ChangeFieldResCount && c.ChangeType == changes.ChangeTypeUpdated {
		return isResourceCountReduction(c.OldValue, c.NewValue)
	}
	return false
}

func isResourceCountReduction(oldVal, newVal *string) bool {
	if oldVal == nil || newVal == nil {
		return false
	}
	old, err1 := strconv.Atoi(*oldVal)
	cur, err2 := strconv.Atoi(*newVal)
	return err1 == nil && err2 == nil && cur < old
}

func splitRemovalChanges(
	appChanges []application.ApplicationChange,
) (removals, others []application.ApplicationChange) {
	removals = make([]application.ApplicationChange, constants.DefaultInitValue, len(appChanges))
	others = make([]application.ApplicationChange, constants.DefaultInitValue, len(appChanges))
	for i := range appChanges {
		if isRemovalChange(appChanges[i]) {
			removals = append(removals, appChanges[i])
		} else {
			others = append(others, appChanges[i])
		}
	}
	return removals, others
}

func removalMatchesAddition(pending, current application.ApplicationChange) bool {
	if pending.Field != current.Field {
		return false
	}
	if pending.ChangeType == changes.ChangeTypeRemoved && current.ChangeType == changes.ChangeTypeAdded {
		return derefChangeValue(pending.OldValue) == derefChangeValue(current.NewValue)
	}
	if pending.Field == changes.ChangeFieldResCount &&
		pending.ChangeType == changes.ChangeTypeUpdated &&
		current.ChangeType == changes.ChangeTypeUpdated {
		return derefChangeValue(pending.OldValue) == derefChangeValue(current.NewValue) &&
			derefChangeValue(pending.NewValue) == derefChangeValue(current.OldValue)
	}
	return false
}

func derefChangeValue(p *string) string {
	if p == nil {
		return constants.EmptyString
	}
	return *p
}

func (b *stabilizationBuffer) resolve(
	appName string,
	currentChanges []application.ApplicationChange,
	now time.Time,
) (promoted []application.ApplicationChange, promotedAt time.Time, filtered []application.ApplicationChange) {
	b.mu.Lock()
	defer b.mu.Unlock()

	p := b.pending[appName]
	if p == nil {
		return nil, time.Time{}, currentChanges
	}

	if now.Sub(p.detectedAt) >= constants.RemovalStabilizationWindow {
		promoted = p.changes
		promotedAt = p.detectedAt
		delete(b.pending, appName)
		return promoted, promotedAt, currentChanges
	}

	remaining, suppressSet := matchReappearances(p.changes, currentChanges)
	if len(remaining) == constants.DefaultInitValue {
		delete(b.pending, appName)
	} else {
		p.changes = remaining
	}

	filtered = make([]application.ApplicationChange, constants.DefaultInitValue, len(currentChanges))
	for i, c := range currentChanges {
		if !suppressSet[i] {
			filtered = append(filtered, c)
		}
	}
	return nil, time.Time{}, filtered
}

func matchReappearances(
	pendingChanges []application.ApplicationChange,
	currentChanges []application.ApplicationChange,
) (remaining []application.ApplicationChange, suppressSet map[int]bool) {
	remaining = make([]application.ApplicationChange, constants.DefaultInitValue, len(pendingChanges))
	suppressSet = make(map[int]bool)
	for _, pending := range pendingChanges {
		matched := false
		for i, current := range currentChanges {
			if suppressSet[i] {
				continue
			}
			if removalMatchesAddition(pending, current) {
				matched = true
				suppressSet[i] = true
				break
			}
		}
		if !matched {
			remaining = append(remaining, pending)
		}
	}
	return remaining, suppressSet
}

func (b *stabilizationBuffer) add(appName string, removals []application.ApplicationChange, detectedAt time.Time) {
	if len(removals) == constants.DefaultInitValue {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	existing := b.pending[appName]
	if existing == nil {
		b.pending[appName] = &pendingRemoval{
			changes:    removals,
			detectedAt: detectedAt,
		}
		return
	}
	pendingSet := make(map[changeKey]bool, len(existing.changes))
	for _, c := range existing.changes {
		pendingSet[signatureOf(c)] = true
	}
	for _, c := range removals {
		if !pendingSet[signatureOf(c)] {
			existing.changes = append(existing.changes, c)
		}
	}
}

func mergePromotedAndImmediate(
	promoted, immediate []application.ApplicationChange,
) []application.ApplicationChange {
	if len(promoted) == constants.DefaultInitValue {
		return immediate
	}
	if len(immediate) == constants.DefaultInitValue {
		return promoted
	}
	out := make([]application.ApplicationChange, constants.DefaultInitValue, len(promoted)+len(immediate))
	out = append(out, promoted...)
	out = append(out, immediate...)
	return out
}
