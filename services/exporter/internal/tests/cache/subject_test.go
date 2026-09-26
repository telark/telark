package cache

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/telark/exporter/internal/cache"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/utils/performance"
	categoryendpoints "github.com/telark/rest/endpoints/categories"
)

const (
	subjectA = "u-00001-0001-000a"
	subjectB = "u-00001-0001-000b"

	listPath = "/api/v1/list"

	wantOneCall  = 1
	wantTwoCalls = 2
)

func newOptimizer(t *testing.T) *performance.Optimizer {
	t.Helper()
	mr := miniredis.RunT(t)
	o := performance.NewOptimizer(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	t.Cleanup(o.Close)
	return o
}

func echoSubjectHandler(subject func(*http.Request) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"data":{"owner":%q}}`, subject(r))
	}
}

func headerRequest(value string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, listPath, nil)
	r.Header.Set(constants.HeaderUserID, value)
	return r
}

// A per-subject list is served from the cache to the next caller, so the key
// has to carry the subject or one subject is handed another subject's rows.
func TestPerSubjectListCacheIsNotSharedAcrossSubjects(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		subject      cache.SubjectFunc
		request      func(value string) *http.Request
	}{
		{
			name:         "passkeys by user",
			resourceType: constants.ResourceUserPasskey,
			subject:      cache.SubjectFromHeader(constants.HeaderUserID),
			request:      headerRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := newOptimizer(t)
			handler := performance.NewCachedListHandlerFunc(
				o,
				echoSubjectHandler(tt.subject),
				cache.NewSubjectListCacheKeyFunc(o, tt.resourceType, tt.subject),
				tt.resourceType,
				constants.OpList,
			)

			first := httptest.NewRecorder()
			handler(first, tt.request(subjectA))
			if !strings.Contains(first.Body.String(), subjectA) {
				t.Fatalf("first caller got %q, want it to contain %q", first.Body.String(), subjectA)
			}

			second := httptest.NewRecorder()
			handler(second, tt.request(subjectB))
			body := second.Body.String()
			if strings.Contains(body, subjectA) {
				t.Errorf("second caller was served the first caller's rows: %q", body)
			}
			if !strings.Contains(body, subjectB) {
				t.Errorf("second caller got %q, want it to contain %q", body, subjectB)
			}
		})
	}
}

func countingHandler(calls *int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		*calls++
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"data":{"calls":%d}}`, *calls)
	}
}

// A subject-scoped key is only safe if invalidation still reaches it; a writer
// that cannot know which subjects are affected must clear all of them.
func TestListInvalidationReachesPerSubjectEntries(t *testing.T) {
	tests := []struct {
		name       string
		invalidate func(o *performance.Optimizer)
	}{
		{
			name: "smart invalidate on create",
			invalidate: func(o *performance.Optimizer) {
				cache.SmartInvalidateListCache(o, constants.ResourceUserSession, constants.OpCreate)
			},
		},
		{
			name: "invalidate list cache",
			invalidate: func(o *performance.Optimizer) {
				cache.InvalidateListCache(o, constants.ResourceUserSession)
			},
		},
		{
			name: "invalidate specific resource",
			invalidate: func(o *performance.Optimizer) {
				cache.InvalidateSpecificResourceCache(o, constants.ResourceUserSession, subjectA)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := newOptimizer(t)
			calls := constants.DefaultInitValue
			subject := cache.SubjectFromHeader(constants.HeaderUserID)
			handler := performance.NewCachedListHandlerFunc(
				o,
				countingHandler(&calls),
				cache.NewSubjectListCacheKeyFunc(o, constants.ResourceUserSession, subject),
				constants.ResourceUserSession,
				constants.OpList,
			)

			handler(httptest.NewRecorder(), headerRequest(subjectA))
			handler(httptest.NewRecorder(), headerRequest(subjectA))
			if calls != wantOneCall {
				t.Fatalf("handler ran %d times, want %d — the second read was not cached", calls, wantOneCall)
			}

			tt.invalidate(o)
			handler(httptest.NewRecorder(), headerRequest(subjectA))
			if calls != wantTwoCalls {
				t.Errorf("handler ran %d times, want %d — the per-subject entry survived invalidation", calls, wantTwoCalls)
			}
		})
	}
}

// A request whose subject cannot be read must never fall back to a shared key.
func TestUnresolvableSubjectIsNeverCached(t *testing.T) {
	o := newOptimizer(t)
	calls := constants.DefaultInitValue
	handler := performance.NewCachedListHandlerFunc(
		o,
		countingHandler(&calls),
		cache.NewSubjectListCacheKeyFunc(o, constants.ResourceUserSession, cache.SubjectFromHeader(constants.HeaderUserID)),
		constants.ResourceUserSession,
		constants.OpList,
	)

	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, listPath, nil))
	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, listPath, nil))

	if calls != wantTwoCalls {
		t.Errorf("handler ran %d times, want %d — a subjectless request was served from cache", calls, wantTwoCalls)
	}
}

// Invalidating one resource type must not throw away another type's lists.
func TestListInvalidationIsScopedToItsResourceType(t *testing.T) {
	o := newOptimizer(t)
	calls := constants.DefaultInitValue
	handler := performance.NewCachedListHandlerFunc(
		o,
		countingHandler(&calls),
		cache.NewListCacheKeyFunc(o, constants.ResourceApplication),
		constants.ResourceApplication,
		constants.OpList,
	)

	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, listPath, nil))
	cache.InvalidateListCache(o, constants.ResourceRole)
	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, listPath, nil))

	if calls != wantOneCall {
		t.Errorf("handler ran %d times, want %d — a roles write dropped the applications list", calls, wantOneCall)
	}
}

type staticGeneration string

func (g staticGeneration) ListGeneration(string) string { return string(g) }

func TestListCacheKeysCarryTheirSubject(t *testing.T) {
	const gen = staticGeneration("7")

	tests := []struct {
		name    string
		keyFunc func(*http.Request) string
		request *http.Request
		want    string
	}{
		{
			name:    "global list has no subject",
			keyFunc: cache.NewListCacheKeyFunc(gen, constants.ResourceRole),
			request: httptest.NewRequest(http.MethodGet, listPath, nil),
			want:    "list:accessroles:7",
		},
		{
			name:    "header subject",
			keyFunc: cache.NewSubjectListCacheKeyFunc(gen, constants.ResourceUserPasskey, cache.SubjectFromHeader(constants.HeaderUserID)),
			request: headerRequest(subjectA),
			want:    "list:user-passkeys:7:X-User-ID:" + subjectA,
		},
		{
			name:    "unfiltered query list shares the plain list key",
			keyFunc: cache.NewQueryListCacheKeyFunc(gen, constants.ResourceCategory, categoryendpoints.QueryScope),
			request: httptest.NewRequest(http.MethodGet, listPath, nil),
			want:    "list:categories:7",
		},
		{
			name:    "query value is part of the key",
			keyFunc: cache.NewQueryListCacheKeyFunc(gen, constants.ResourceCategory, categoryendpoints.QueryScope),
			request: httptest.NewRequest(http.MethodGet, listPath+"?scope=groups", nil),
			want:    "list:categories:7:scope:groups",
		},
		{
			name:    "missing subject yields no key at all",
			keyFunc: cache.NewSubjectListCacheKeyFunc(gen, constants.ResourceUserSession, cache.SubjectFromHeader(constants.HeaderUserID)),
			request: httptest.NewRequest(http.MethodGet, listPath, nil),
			want:    constants.EmptyString,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.keyFunc(tt.request); got != tt.want {
				t.Errorf("key = %q, want %q", got, tt.want)
			}
		})
	}
}
