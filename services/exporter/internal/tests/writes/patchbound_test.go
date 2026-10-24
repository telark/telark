package writes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/rest/router"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/routes"
	"github.com/telark/telark/services/exporter/internal/utils/concurrency"
	"github.com/telark/telark/services/exporter/internal/utils/performance"
)

const (
	identityBody = `{"identities":[{"provider":"passkey","subject":"cred-1"}]}`
	// Long enough for the patch to reach the account's lock, which nothing else holds.
	lockHeldFor = 100 * time.Millisecond
)

// The identity write auth sends, through the exporter's router, answered on the returned channel.
func sendIdentityPatch(ctx context.Context, t *testing.T) <-chan *httptest.ResponseRecorder {
	t.Helper()
	optimizer := performance.NewOptimizer(redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()}))
	t.Cleanup(optimizer.Close)
	handler := router.NewRouter(routes.InitRoutes(optimizer, nil))
	r := httptest.NewRequestWithContext(xauthz.WithIdentity(ctx, xauthz.Identity{Internal: true}), http.MethodPatch,
		usersPrefix+plainUser, strings.NewReader(identityBody))
	answered := make(chan *httptest.ResponseRecorder, constants.DefaultIncrementValue)
	go func() {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		answered <- w
	}()
	return answered
}

// With the lock held, the patch must still be waiting when its caller gives up (gone) or the lock is freed.
func patchBehindTheLock(ctx context.Context, t *testing.T, cancel context.CancelFunc, held, gone bool) *httptest.ResponseRecorder {
	t.Helper()
	if !held {
		if gone {
			cancel()
		}
		return <-sendIdentityPatch(ctx, t)
	}
	lock := concurrency.GetLock(plainUser)
	lock.Lock()
	answered := sendIdentityPatch(ctx, t)
	select {
	case w := <-answered:
		lock.Unlock()
		t.Fatalf("answered %d %s while the account's lock was held", w.Code, w.Body.String())
	case <-time.After(lockHeldFor):
	}
	if gone {
		cancel()
	}
	lock.Unlock()
	return <-answered
}

// A user patch that gets the account's lock only after its caller gave up answers 503 and writes nothing, so an
// identity write queued behind others can't land after a break-glass recovery's hold; a waiting caller gets its write.
func TestAQueuedUserPatchWritesOnlyForAWaitingCaller(t *testing.T) {
	cases := []struct {
		name string
		gone bool
		held bool
		want int
	}{
		{"caller waits", false, true, http.StatusOK},
		{"caller gave up while it waited", true, true, http.StatusServiceUnavailable},
		{"caller gave up, with the lock free", true, false, http.StatusServiceUnavailable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client := adminDirectory(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			w := patchBehindTheLock(ctx, t, cancel, c.held, c.gone)
			if w.Code != c.want {
				t.Fatalf("patch = %d %s, want %d", w.Code, w.Body.String(), c.want)
			}
			if c.want == http.StatusServiceUnavailable && !strings.Contains(w.Body.String(), string(constants.ErrUserPatchBusy)) {
				t.Errorf("503 body %s lacks %q", w.Body.String(), constants.ErrUserPatchBusy)
			}
			patched := slices.ContainsFunc(writes(t, client), func(w write) bool {
				return w.resource == v1alpha1.UserMetadata.Plural && w.name == plainUser
			})
			if patched != (c.want == http.StatusOK) {
				t.Errorf("user patched = %v, want %v", patched, c.want == http.StatusOK)
			}
		})
	}
}
