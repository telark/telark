package performance

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	roledata "github.com/telark/telark/internal/data/resources/role"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/cache"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/utils/performance"
)

const (
	applicationsPath = "/applications"
	parallelLists    = 50
	// parallelLists renders of this length, two at a time, outlast a short fixed
	// wait for a slot, so a queue that sheds early fails the test.
	renderTime   = 100 * time.Millisecond
	shortTimeout = 50 * time.Millisecond
	// Callers queued on one key finish well inside it only by sharing one render;
	// re-rendering one after another at renderTime each outlasts it.
	listDeadline     = time.Second
	userAdmin        = "u-admin"
	userPlain        = "u-plain"
	oversizedCallers = 8
	secondRender     = 2
	listedMessage    = "listed"
	oversizedFill    = "x"
)

func TestGetTimeoutForResource(t *testing.T) {
	// A resource+operation configured explicitly returns its specific timeout.
	if got := performance.GetTimeoutForResource(constants.ResourceApplication, constants.OpGet); got <= constants.DefaultInitValue {
		t.Errorf("configured timeout = %v, want > 0", got)
	}
	// An unconfigured resource falls back to the per-operation default.
	if got := performance.GetTimeoutForResource("unconfigured", constants.OpCreate); got <= constants.DefaultInitValue {
		t.Errorf("fallback timeout = %v, want > 0", got)
	}
	// An unknown operation lands on the global default.
	if got := performance.GetTimeoutForResource("unconfigured", "unknown-op"); got <= constants.DefaultInitValue {
		t.Errorf("default timeout = %v, want > 0", got)
	}
}

func serveList(handler http.HandlerFunc, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func listBody(w http.ResponseWriter, r *http.Request) {
	responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess, listedMessage, map[string]any{"path": r.URL.Path}, nil)
}

// Two renders may run at once by default; a third, on its own key, waits for a
// slot until its own deadline and only then is refused with Retry-After.
func TestCachedListHandlerBoundsConcurrentRenders(t *testing.T) {
	o := newOptimizer(t)
	started := make(chan struct{}, constants.DefaultListRenderConcurrency)
	release := make(chan struct{})
	blocking := func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		<-release
		listBody(w, r)
	}
	keyByPath := func(r *http.Request) string { return "list:test" + r.URL.Path }
	handler := performance.NewCachedListHandlerFunc(o, blocking, keyByPath, constants.ResourceApplication, constants.OpList)

	var wg sync.WaitGroup
	codes := make([]int, constants.DefaultListRenderConcurrency)
	for i := range codes {
		wg.Go(func() {
			codes[i] = serveList(handler, "/"+strconv.Itoa(i)).Code
		})
	}
	for range codes {
		<-started
	}

	ctx, cancel := context.WithTimeout(context.Background(), shortTimeout)
	defer cancel()
	refused := httptest.NewRecorder()
	handler(refused, httptest.NewRequestWithContext(ctx, http.MethodGet, "/beyond", nil))
	if refused.Code != http.StatusServiceUnavailable || refused.Header().Get(constants.HeaderRetryAfter) != constants.ListRenderRetryAfter {
		t.Fatalf("beyond capacity: code = %d, Retry-After = %q", refused.Code, refused.Header().Get(constants.HeaderRetryAfter))
	}
	if refused.Header().Get(constants.HeaderETag) != constants.EmptyString {
		t.Error("refused response carries a validator")
	}
	close(release)
	wg.Wait()
	for i, code := range codes {
		if code != http.StatusOK {
			t.Errorf("render %d: code = %d, want 200", i, code)
		}
	}
}

// Worst case, every request on its own key: renders stay at the slot count and
// the rest queue for a slot instead of being refused.
func TestCachedListHandlerQueuesParallelListsWithout503(t *testing.T) {
	o := newOptimizer(t)
	var mu sync.Mutex
	running, peak := constants.DefaultInitValue, constants.DefaultInitValue
	slow := func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		running++
		peak = max(peak, running)
		mu.Unlock()
		time.Sleep(renderTime)
		mu.Lock()
		running--
		mu.Unlock()
		listBody(w, r)
	}
	keyByPath := func(r *http.Request) string { return "list:test" + r.URL.Path }
	handler := performance.NewCachedListHandlerFunc(o, slow, keyByPath, constants.ResourceApplication, constants.OpList)

	var wg sync.WaitGroup
	codes := make([]int, parallelLists)
	for i := range codes {
		wg.Go(func() {
			codes[i] = serveList(handler, "/"+strconv.Itoa(i)).Code
		})
	}
	wg.Wait()
	for i, code := range codes {
		if code != http.StatusOK {
			t.Errorf("request %d: code = %d, want 200", i, code)
		}
	}
	if peak > constants.DefaultListRenderConcurrency {
		t.Errorf("peak renders = %d, want at most %d", peak, constants.DefaultListRenderConcurrency)
	}
}

// Restricted callers all see the same filtered list, so parallel loads by
// different restricted users are one render, not one each.
func TestRestrictedListCallersShareOneRender(t *testing.T) {
	o := newOptimizer(t)
	var renders atomic.Int32
	counting := func(w http.ResponseWriter, r *http.Request) {
		renders.Add(int32(constants.DefaultIncrementValue))
		time.Sleep(renderTime)
		listBody(w, r)
	}
	key := authz.RestrictedListKey(o, cache.NewListCacheKeyFunc(o, constants.ResourceUser))
	handler := performance.NewCachedListHandlerFunc(o, counting, key, constants.ResourceUser, constants.OpList)

	var wg sync.WaitGroup
	codes := make([]int, parallelLists)
	for i := range codes {
		wg.Go(func() {
			identity := xauthz.Identity{
				UserID: "u-" + strconv.Itoa(i),
				Grants: xauthz.Grants{Levels: map[string]roledata.PermissionLevel{roledata.ScopeUsers: roledata.PermissionLevelOwner}},
			}
			r := httptest.NewRequest(http.MethodGet, "/users", nil)
			rec := httptest.NewRecorder()
			handler(rec, r.WithContext(xauthz.WithIdentity(r.Context(), identity)))
			codes[i] = rec.Code
		})
	}
	wg.Wait()
	for i, code := range codes {
		if code != http.StatusOK {
			t.Errorf("request %d: code = %d, want 200", i, code)
		}
	}
	if got := renders.Load(); got != int32(constants.DefaultIncrementValue) {
		t.Errorf("renders = %d, want 1", got)
	}
}

// A repeat under the current key is served from the in-process blob: the Redis
// copy is deleted between the requests and the handler still renders once. The
// hit is spliced into the same envelope the live response used.
func TestCachedListHandlerServesLocalBlobWithoutRedis(t *testing.T) {
	o := newOptimizer(t)
	renders := constants.DefaultInitValue
	live := func(w http.ResponseWriter, r *http.Request) {
		renders++
		listBody(w, r)
	}
	keyFunc := cache.NewListCacheKeyFunc(o, constants.ResourceApplication)
	handler := performance.NewCachedListHandlerFunc(o, live, keyFunc, constants.ResourceApplication, constants.OpList)

	first := serveList(handler, applicationsPath)
	o.Delete(keyFunc(httptest.NewRequest(http.MethodGet, applicationsPath, nil)))
	second := serveList(handler, applicationsPath)
	if renders != constants.DefaultIncrementValue || second.Code != http.StatusOK {
		t.Fatalf("renders = %d, second code = %d, want one render and a local hit", renders, second.Code)
	}

	var liveResp, hitResp response.GenericResponse
	if err := json.Unmarshal(first.Body.Bytes(), &liveResp); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(second.Body.Bytes(), &hitResp); err != nil {
		t.Fatalf("cached body is not a response envelope: %v: %s", err, second.Body.String())
	}
	if hitResp.Status != liveResp.Status || hitResp.Operation != liveResp.Operation || !reflect.DeepEqual(hitResp.Data, liveResp.Data) {
		t.Errorf("hit = %+v, live = %+v", hitResp, liveResp)
	}
	if hitResp.Message != constants.CachedResponse {
		t.Errorf("hit message = %q, want %q", hitResp.Message, constants.CachedResponse)
	}
	if got := second.Header().Get(constants.HeaderContentLength); got != strconv.Itoa(second.Body.Len()) {
		t.Errorf("Content-Length = %q, body = %d bytes", got, second.Body.Len())
	}
	if got := second.Header().Get(constants.HeaderContentType); got != constants.ContentTypeJSON {
		t.Errorf("Content-Type = %q, want %q", got, constants.ContentTypeJSON)
	}
}

// Each IAM write promotes one user to administrator and moves one of the
// generations the restricted list key reads.
var iamWrites = []struct{ resource, user string }{
	{constants.ResourceUser, "u-self"},
	{constants.ResourceGroup, "u-member"},
	{constants.ResourceRole, "u-holder"},
}

var listPersonas = []xauthz.Identity{
	{UserID: userAdmin, Grants: grantsOf(roledata.ScopeAll, roledata.PermissionLevelAdmin)},
	{Internal: true},
	{UserID: "u-users-owner", Grants: grantsOf(roledata.ScopeUsers, roledata.PermissionLevelOwner)},
	{UserID: "u-groups-ro", Grants: grantsOf(roledata.ScopeGroups, roledata.PermissionLevelReadOnly)},
	{UserID: "u-all-ro", Grants: grantsOf(roledata.ScopeAll, roledata.PermissionLevelReadOnly)},
}

func grantsOf(scope string, level roledata.PermissionLevel) xauthz.Grants {
	return xauthz.Grants{Levels: map[string]roledata.PermissionLevel{scope: level}}
}

func promotedUsers() []string {
	users := make([]string, constants.DefaultInitValue, len(iamWrites))
	for _, write := range iamWrites {
		users = append(users, write.user)
	}
	return users
}

func visibleUsers(restricted bool, promoted []string) []string {
	users := append([]string{userAdmin, userPlain}, promotedUsers()...)
	if !restricted {
		return users
	}
	return slices.DeleteFunc(users, func(id string) bool { return id == userAdmin || slices.Contains(promoted, id) })
}

// Every first render has an IAM write land mid-render, so its key has moved by
// the time it finishes: the callers queued on it still get its body before their
// deadline, and a caller arriving after the writes never sees the promoted users.
func TestListRenderSharedWhenIAMWriteMovesGeneration(t *testing.T) {
	o := newOptimizer(t)
	var (
		mu       sync.Mutex
		promoted []string
		pending  = slices.Clone(iamWrites)
		keys     atomic.Int32
	)
	keyed := make(chan struct{})
	allKeyed := sync.OnceFunc(func() { close(keyed) })
	render := func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		visible := visibleUsers(authz.Restricted(r), promoted)
		writes := slices.Clone(pending[:min(len(pending), constants.DefaultIncrementValue)])
		pending = pending[len(writes):]
		mu.Unlock()
		select {
		case <-keyed:
		case <-r.Context().Done():
		}
		for _, write := range writes {
			mu.Lock()
			promoted = append(promoted, write.user)
			mu.Unlock()
			o.BumpListGeneration(write.resource)
		}
		time.Sleep(renderTime)
		responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess, listedMessage, visible, nil)
	}
	// The writes wait until every first-wave caller holds its key.
	counted := func(key func(*http.Request) string) func(*http.Request) string {
		return func(r *http.Request) string {
			k := key(r)
			if keys.Add(int32(constants.DefaultIncrementValue)) >= parallelLists {
				allKeyed()
			}
			return k
		}
	}
	routes := []http.HandlerFunc{
		performance.NewCachedListHandlerFunc(o, render, counted(authz.RestrictedListKey(o, cache.NewListCacheKeyFunc(o, constants.ResourceUser))),
			constants.ResourceUser, constants.OpList),
		performance.NewCachedListHandlerFunc(o, render, counted(authz.RestrictedListKey(o, cache.NewListCacheKeyFunc(o, constants.ResourceGroup))),
			constants.ResourceGroup, constants.OpList),
	}

	checkListWave(t, routes, parallelLists, nil)
	checkListWave(t, routes, len(routes)*len(listPersonas), promotedUsers())
}

func checkListWave(t *testing.T, routes []http.HandlerFunc, n int, promoted []string) {
	t.Helper()
	recs := make([]*httptest.ResponseRecorder, n)
	took := make([]time.Duration, n)
	restricted := make([]bool, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), listDeadline)
			defer cancel()
			r := httptest.NewRequestWithContext(xauthz.WithIdentity(ctx, listPersonas[i%len(listPersonas)]), http.MethodGet, "/list", nil)
			restricted[i] = authz.Restricted(r)
			recs[i] = httptest.NewRecorder()
			begin := time.Now()
			routes[i%len(routes)](recs[i], r)
			took[i] = time.Since(begin)
		})
	}
	wg.Wait()
	for i, rec := range recs {
		var body struct {
			Data []string `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); rec.Code != http.StatusOK || err != nil || took[i] >= listDeadline {
			t.Errorf("request %d: code = %d after %v, decode error = %v", i, rec.Code, took[i], err)
			continue
		}
		leaked := slices.ContainsFunc(promoted, func(id string) bool { return slices.Contains(body.Data, id) })
		if restricted[i] == slices.Contains(body.Data, userAdmin) || !slices.Contains(body.Data, userPlain) || restricted[i] && leaked {
			t.Errorf("request %d (restricted %t): users = %v", i, restricted[i], body.Data)
		}
	}
}

// A caller queued behind another caller's render of the same key gives up at
// its own deadline instead of waiting the render out.
func TestCoalescedWaiterHonoursDeadline(t *testing.T) {
	o := newOptimizer(t)
	started, release := make(chan struct{}), make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	time.AfterFunc(2*listDeadline, unblock)
	blocking := func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		listBody(w, r)
	}
	shared := func(*http.Request) string { return "list:test:shared" }
	handler := performance.NewCachedListHandlerFunc(o, blocking, shared, constants.ResourceApplication, constants.OpList)

	var wg sync.WaitGroup
	wg.Go(func() { serveList(handler, applicationsPath) })
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), shortTimeout)
	defer cancel()
	waiter := httptest.NewRecorder()
	begin := time.Now()
	handler(waiter, httptest.NewRequestWithContext(ctx, http.MethodGet, applicationsPath, nil))
	took := time.Since(begin)
	unblock()
	wg.Wait()
	if waiter.Code != http.StatusServiceUnavailable || waiter.Header().Get(constants.HeaderRetryAfter) != constants.ListRenderRetryAfter ||
		took >= listDeadline {
		t.Errorf("waiter: code = %d, Retry-After = %q after %v", waiter.Code, waiter.Header().Get(constants.HeaderRetryAfter), took)
	}
}

// A list too large to cache is still handed to every caller queued on its
// render, and never stored: the next caller renders it again.
func TestOversizedListSharedButNotStored(t *testing.T) {
	o := newOptimizer(t)
	payload := strings.Repeat(oversizedFill, performance.MaxResponseSize)
	var renders, keys atomic.Int32
	keyed := make(chan struct{})
	allKeyed := sync.OnceFunc(func() { close(keyed) })
	render := func(w http.ResponseWriter, r *http.Request) {
		renders.Add(int32(constants.DefaultIncrementValue))
		select {
		case <-keyed:
		case <-r.Context().Done():
		}
		time.Sleep(renderTime)
		responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess, listedMessage, payload, nil)
	}
	key := func(*http.Request) string {
		if keys.Add(int32(constants.DefaultIncrementValue)) >= oversizedCallers {
			allKeyed()
		}
		return "list:test:oversized"
	}
	handler := performance.NewCachedListHandlerFunc(o, render, key, constants.ResourceApplication, constants.OpList)

	var wg sync.WaitGroup
	sizes := make([]int, oversizedCallers, oversizedCallers+secondRender)
	for i := range sizes {
		wg.Go(func() { sizes[i] = oversizedDataSize(t, handler) })
	}
	wg.Wait()
	if got := renders.Load(); got != int32(constants.DefaultIncrementValue) {
		t.Errorf("renders = %d, want 1 shared by %d callers", got, oversizedCallers)
	}
	sizes = append(sizes, oversizedDataSize(t, handler))
	for i, size := range sizes {
		if size != len(payload) {
			t.Errorf("caller %d: data = %d bytes, want %d", i, size, len(payload))
		}
	}
	if got := renders.Load(); got != secondRender {
		t.Errorf("renders after a later call = %d, want 2: an oversized list is not stored", got)
	}
}

func oversizedDataSize(t *testing.T, handler http.HandlerFunc) int {
	t.Helper()
	rec := serveList(handler, applicationsPath)
	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", rec.Code)
	}
	return bytes.Count(rec.Body.Bytes(), []byte(oversizedFill))
}

// A request whose deadline has passed never takes a free render slot.
func TestExpiredRequestNeverRenders(t *testing.T) {
	o := newOptimizer(t)
	var renders atomic.Int32
	counting := func(w http.ResponseWriter, r *http.Request) {
		renders.Add(int32(constants.DefaultIncrementValue))
		listBody(w, r)
	}
	expired := func(*http.Request) string { return "list:test:expired" }
	handler := performance.NewCachedListHandlerFunc(o, counting, expired, constants.ResourceApplication, constants.OpList)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for i := range parallelLists {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, "/"+strconv.Itoa(i), nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("request %d: code = %d, want 503", i, rec.Code)
		}
	}
	if got := renders.Load(); got != int32(constants.DefaultInitValue) {
		t.Errorf("renders = %d, want 0", got)
	}
}
