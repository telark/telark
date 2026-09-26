package reshandlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	basemetadata "github.com/telark/data/metadata/base"
	metadata "github.com/telark/data/metadata/v1alpha1"
	"github.com/telark/exporter/internal/cache"
	"github.com/telark/exporter/internal/constants"
	resshared "github.com/telark/exporter/internal/handlers/resources/shared"
	authshared "github.com/telark/exporter/internal/utils/auth/shared"
	categoryutils "github.com/telark/exporter/internal/utils/classification/category"
	"github.com/telark/exporter/internal/utils/performance"
)

const (
	namedJSONBody   = `{"name":"` + testAppName + `"}`
	wantGeneration  = "1"
	writeResourceTy = constants.ResourceApplication
)

func noopDelete(http.ResponseWriter, basemetadata.Metadata, string) {}

func namedReq(method string) *http.Request {
	r := httptest.NewRequest(method, "/resource/"+testAppName, strings.NewReader(emptyJSONBody))
	return mux.SetURLVars(r, map[string]string{constants.NameParam: testAppName})
}

// A second bump inside the coalescing window leaves the dirty flag behind, so
// the first reader after the window pays a spurious full rebuild.
func TestOneListGenerationBumpPerWrite(t *testing.T) {
	md := metadata.ApplicationMetadata
	cases := []struct {
		name         string
		resourceType string
		write        func(o *performance.Optimizer)
	}{
		{"shared create", writeResourceTy, func(o *performance.Optimizer) {
			r := httptest.NewRequest(http.MethodPost, "/resource", strings.NewReader(namedJSONBody))
			resshared.CreateResourceWithCacheInvalidation(o, md, writeResourceTy)(httptest.NewRecorder(), r)
		}},
		{"shared patch", writeResourceTy, func(o *performance.Optimizer) {
			resshared.PatchResourceWithCacheInvalidation(o, md, writeResourceTy)(httptest.NewRecorder(), namedReq(http.MethodPatch))
		}},
		{"shared delete", writeResourceTy, func(o *performance.Optimizer) {
			resshared.DeleteResourceWithCacheInvalidation(o, md, writeResourceTy, noopDelete)(httptest.NewRecorder(), namedReq(http.MethodDelete))
		}},
		{"passkey create", constants.ResourceUserPasskey, func(o *performance.Optimizer) {
			authshared.InvalidateResourceCaches(o, constants.ResourceUserPasskey, constants.OpCreate, constants.EmptyString)
		}},
		{"session delete", constants.ResourceUserSession, func(o *performance.Optimizer) {
			authshared.InvalidateResourceCaches(o, constants.ResourceUserSession, constants.OpDelete, constants.EmptyString)
		}},
		{"category write", constants.ResourceCategory, func(o *performance.Optimizer) {
			categoryutils.InvalidateCategoryCaches(o)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mr := miniredis.RunT(t)
			o := performance.NewOptimizer(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
			t.Cleanup(o.Close)

			tc.write(o)

			if got, _ := mr.Get(cache.ListGenerationKey(tc.resourceType)); got != wantGeneration {
				t.Errorf("generation = %q, want %q", got, wantGeneration)
			}
			if mr.Exists(cache.ListGenerationDirtyKey(tc.resourceType)) {
				t.Error("a second bump was deferred into the dirty flag")
			}
		})
	}
}
