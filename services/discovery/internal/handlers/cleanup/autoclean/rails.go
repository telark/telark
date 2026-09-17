package autoclean

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	appresource "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	gcfghelper "github.com/telark/discovery/internal/helpers/globalconfig"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// railResult captures the outcome of one rail check.
type railResult struct {
	name    string
	pass    bool
	reason  string // when pass=false, human-readable explanation for logs
	hardErr error  // set when the rail itself failed (Redis down, etc.) — caller skips streak update
}

func pass(name string) railResult { return railResult{name: name, pass: true} }
func block(name, reason string) railResult {
	return railResult{name: name, pass: false, reason: reason}
}

func errored(name string, err error) railResult {
	return railResult{name: name, pass: false, reason: err.Error(), hardErr: err}
}

// railResourcesEmpty — app's tracked resources report empty.
func railResourcesEmpty(app *appresource.Application) railResult {
	if app == nil {
		return block(constants.RailResourcesEmpty, "app nil")
	}
	if app.ResourceCount != constants.DefaultInitValue {
		return block(constants.RailResourcesEmpty,
			fmt.Sprintf("ResourceCount=%d", app.ResourceCount))
	}
	if len(app.Resources) != constants.DefaultInitValue {
		return block(constants.RailResourcesEmpty,
			fmt.Sprintf("len(Resources)=%d", len(app.Resources)))
	}
	return pass(constants.RailResourcesEmpty)
}

// railNoActiveRollback — no rollback in pending/in_progress state.
func railNoActiveRollback(app *appresource.Application) railResult {
	if app == nil {
		return pass(constants.RailNoActiveRollback)
	}
	for i := range app.Rollbacks {
		s := app.Rollbacks[i].Status
		if s == constants.RollbackStatusPending || s == constants.RollbackStatusInProgress {
			return block(constants.RailNoActiveRollback,
				fmt.Sprintf("rollback %s status=%s", app.Rollbacks[i].ID, s))
		}
	}
	return pass(constants.RailNoActiveRollback)
}

// railNamespaceExists — every namespace owned by the app exists and is not terminating.
func railNamespaceExists(ctx context.Context, kube *kubernetes.Clientset, app *appresource.Application) railResult {
	if app == nil || kube == nil {
		return pass(constants.RailNamespaceExists)
	}
	for i := range app.Namespaces.Items {
		ns := app.Namespaces.Items[i].Name
		got, err := kube.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
		if err != nil {
			// A deleted namespace took the app's resources with it: nothing left to wait for.
			if k8serrors.IsNotFound(err) {
				continue
			}
			return errored(constants.RailNamespaceExists, err)
		}
		if got.DeletionTimestamp != nil {
			return block(constants.RailNamespaceExists,
				fmt.Sprintf("namespace %s is terminating", ns))
		}
	}
	return pass(constants.RailNamespaceExists)
}

// namespacesGone — every namespace the app owns is deleted. The informers stop
// with the namespace, so the stored resource list is stale and must be ignored.
func namespacesGone(ctx context.Context, kube *kubernetes.Clientset, app *appresource.Application) bool {
	if app == nil || kube == nil || len(app.Namespaces.Items) == constants.DefaultInitValue {
		return false
	}
	for i := range app.Namespaces.Items {
		_, err := kube.CoreV1().Namespaces().Get(ctx, app.Namespaces.Items[i].Name, metav1.GetOptions{})
		if !k8serrors.IsNotFound(err) {
			return false
		}
	}
	return true
}

// railNamespaceIncluded — none of the app's namespaces are in the excluded list.
func railNamespaceIncluded(ctx context.Context, app *appresource.Application) railResult {
	if app == nil {
		return pass(constants.RailNamespaceIncluded)
	}
	excluded := gcfghelper.FetchExcludedNamespaces(ctx)
	if len(excluded) == constants.DefaultInitValue {
		return pass(constants.RailNamespaceIncluded)
	}
	exSet := make(map[string]struct{}, len(excluded))
	for _, e := range excluded {
		exSet[e] = struct{}{}
	}
	for i := range app.Namespaces.Items {
		ns := app.Namespaces.Items[i].Name
		if _, ok := exSet[ns]; ok {
			return block(constants.RailNamespaceIncluded,
				fmt.Sprintf("namespace %s is in excluded list", ns))
		}
	}
	return pass(constants.RailNamespaceIncluded)
}

// railRedisKeyAbsent — generic helper: blocks if EXISTS returns >0.
func railRedisKeyAbsent(
	ctx context.Context,
	rdb redis.Cmdable,
	railName, key string,
) railResult {
	if rdb == nil {
		return pass(railName)
	}
	cctx, cancel := context.WithTimeout(ctx, constants.AutoCleanupRailReadTimeout)
	defer cancel()
	n, err := rdb.Exists(cctx, key).Result()
	if err != nil {
		return errored(railName, err)
	}
	if n > constants.DefaultInitValue {
		return block(railName, "redis key present: "+key)
	}
	return pass(railName)
}

// railNoForceSync — no force-sync lock or dedup key held.
func railNoForceSync(ctx context.Context, rdb redis.Cmdable, appName string) railResult {
	if r := railRedisKeyAbsent(ctx, rdb, constants.RailNoForceSync,
		constants.KeyPrefixLockApp+appName); !r.pass {
		return r
	}
	return railRedisKeyAbsent(ctx, rdb, constants.RailNoForceSync,
		constants.ForceSyncDedupKeyPrefix+appName)
}

// railNoCoalesceBuffer — no pending coalesce buffer entry.
func railNoCoalesceBuffer(ctx context.Context, rdb redis.Cmdable, appName string) railResult {
	return railRedisKeyAbsent(ctx, rdb, constants.RailNoCoalesceBuffer,
		constants.KeyPrefixCoalesceBuffer+appName)
}

// railNoEnrichmentLock — no enrichment lock held.
func railNoEnrichmentLock(ctx context.Context, rdb redis.Cmdable, appName string) railResult {
	return railRedisKeyAbsent(ctx, rdb, constants.RailNoEnrichmentLock,
		constants.KeyPrefixLockEnrich+appName)
}

// railNoGenerationLock — no per-generation processing lock held.
// Uses SCAN with pattern: cardinality per app is small (current gen only).
func railNoGenerationLock(ctx context.Context, rdb redis.Cmdable, appName string) railResult {
	if rdb == nil {
		return pass(constants.RailNoGenerationLock)
	}
	cctx, cancel := context.WithTimeout(ctx, constants.AutoCleanupRailReadTimeout)
	defer cancel()
	pattern := constants.KeyPrefixLockGen + appName + ":*"
	keys, _, err := rdb.Scan(cctx, 0, pattern, 10).Result()
	if err != nil {
		return errored(constants.RailNoGenerationLock, err)
	}
	if len(keys) > constants.DefaultInitValue {
		return block(constants.RailNoGenerationLock,
			fmt.Sprintf("generation lock(s) held: %v", keys))
	}
	return pass(constants.RailNoGenerationLock)
}

// railCleanupCooldown — no recent manual cleanup attempt cooling down.
func railCleanupCooldown(ctx context.Context, rdb redis.Cmdable, appName string) railResult {
	return railRedisKeyAbsent(ctx, rdb, constants.RailCleanupCooldown,
		constants.KeyPrefixResetCooldown+appName)
}

// railSustainedAbsence — empty streak count meets threshold.
func railSustainedAbsence(s streakState, required int) railResult {
	if s.Count < required {
		return block(constants.RailSustainedAbsence,
			fmt.Sprintf("emptyCycles=%d < required=%d", s.Count, required))
	}
	return pass(constants.RailSustainedAbsence)
}

// railGracePeriod — wall-clock delay since first-empty observation meets threshold.
func railGracePeriod(s streakState, grace time.Duration, now time.Time) railResult {
	if s.FirstEmptyUnixMs == int64(constants.DefaultInitValue) {
		return block(constants.RailGracePeriod, "first-empty timestamp unset")
	}
	first := time.UnixMilli(s.FirstEmptyUnixMs).UTC()
	if now.Sub(first) < grace {
		return block(constants.RailGracePeriod,
			fmt.Sprintf("elapsed=%s < grace=%s", now.Sub(first), grace))
	}
	return pass(constants.RailGracePeriod)
}
