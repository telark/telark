package performance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/telark/exporter/internal/constants"
	restconstants "github.com/telark/rest/constants"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

var (
	lg                   = constants.GetLogger(constants.PrefixOptimizer)
	errInvalidCachedJSON = errors.New("cached payload is not valid JSON")
)

func NewOptimizedHandler(optimizer *Optimizer, handler http.HandlerFunc, timeout time.Duration) *OptimizedHandler {
	return &OptimizedHandler{
		optimizer: optimizer,
		handler:   handler,
		timeout:   timeout,
	}
}

func NewDynamicOptimizedHandlerFunc(
	optimizer *Optimizer,
	handler http.HandlerFunc,
	resourceType string,
	operation string,
) func(http.ResponseWriter, *http.Request) {
	timeout := GetTimeoutForResource(resourceType, operation)
	soh := NewOptimizedHandler(optimizer, handler, timeout)
	return func(w http.ResponseWriter, r *http.Request) {
		soh.ServeHTTP(w, r)
	}
}

func NewCachedListHandlerFunc(
	optimizer *Optimizer,
	handler http.HandlerFunc,
	cacheKeyFunc CacheKeyFunc,
	resourceType string,
	operation string,
) func(http.ResponseWriter, *http.Request) {
	clh := NewCachedListHandler(optimizer, handler, cacheKeyFunc, resourceType, operation)
	return func(w http.ResponseWriter, r *http.Request) {
		clh.ServeHTTP(w, r)
	}
}

func (soh *OptimizedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), soh.timeout)
	defer cancel()
	requestID := generateRequestID()
	ctx = context.WithValue(ctx, RequestIDCtxKey, requestID)
	r = r.WithContext(ctx)

	soh.handler(w, r)
}

func (clh *CachedListHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	timeout := GetTimeoutForResource(clh.resourceType, clh.operation)
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	requestID := generateRequestID()
	ctx = context.WithValue(ctx, RequestIDCtxKey, requestID)
	r = r.WithContext(ctx)
	// A caller deriving the next generation from the stored copy asks for a
	// fresh read: CRs are also written outside this service.
	if r.Header.Get(restconstants.HeaderCacheControl) == restconstants.CacheControlNoCache {
		clh.handler(w, r)
		return
	}
	cacheKey := clh.cacheKey(r)

	if clh.serveCached(w, requestID, cacheKey) {
		return
	}

	lg.Info(fmt.Sprintf(string(constants.InfOptimizerCacheMiss), requestID, cacheKey))
	clh.executeHandler(w, r, requestID, cacheKey)
}

func (clh *CachedListHandler) serveCached(w http.ResponseWriter, requestID string, cacheKey string) bool {
	cached, exists := clh.optimizer.Get(cacheKey)
	if !exists {
		return false
	}
	lg.Info(fmt.Sprintf(string(constants.InfOptimizerCacheHit), requestID, cacheKey))
	// The blob is the JSON we stored; embedding it verbatim skips a decode and
	// re-encode of up to a megabyte per request.
	var cachedData any
	switch v := cached.(type) {
	case []byte:
		if !json.Valid(v) {
			lg.Error(fmt.Sprintf(string(constants.ErrOptimizerCacheParseError), requestID, errInvalidCachedJSON))
			return false
		}
		cachedData = json.RawMessage(v)
	case string:
		if !json.Valid([]byte(v)) {
			lg.Error(fmt.Sprintf(string(constants.ErrOptimizerCacheParseError), requestID, errInvalidCachedJSON))
			return false
		}
		cachedData = json.RawMessage(v)
	default:
		cachedData = v
	}
	responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess, constants.CachedResponse, cachedData, nil)
	return true
}

func (clh *CachedListHandler) inflightLock(cacheKey string) *sync.Mutex {
	clh.inflightMu.Lock()
	defer clh.inflightMu.Unlock()
	if lock, ok := clh.inflight[cacheKey]; ok {
		return lock
	}
	lock := &sync.Mutex{}
	clh.inflight[cacheKey] = lock
	return lock
}

func (clh *CachedListHandler) inflightRelease(cacheKey string) {
	clh.inflightMu.Lock()
	defer clh.inflightMu.Unlock()
	delete(clh.inflight, cacheKey)
}

func (clh *CachedListHandler) executeHandler(w http.ResponseWriter, r *http.Request, requestID string, cacheKey string) {
	if cacheKey != constants.EmptyString {
		// Ten concurrent misses on a 1 MB list must not mean ten rebuilds: the
		// first caller builds and stores, the others wait and serve the copy.
		lock := clh.inflightLock(cacheKey)
		lock.Lock()
		defer lock.Unlock()
		defer clh.inflightRelease(cacheKey)
		if clh.serveCached(w, requestID, cacheKey) {
			return
		}
	}
	responseCapture := &responseCaptureWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           make([]byte, constants.DefaultInitValue),
	}

	clh.handler(responseCapture, r)

	if responseCapture.statusCode == http.StatusOK {
		storeResponseInCache(clh, r, responseCapture, cacheKey, requestID)
	} else {
		lg.Info(fmt.Sprintf(string(constants.InfOptimizerCacheStoreSkipped), requestID, responseCapture.statusCode))
	}
}

func (rcw *responseCaptureWriter) WriteHeader(statusCode int) {
	rcw.statusCode = statusCode
	rcw.ResponseWriter.WriteHeader(statusCode)
}

func (rcw *responseCaptureWriter) Write(data []byte) (int, error) {
	if len(rcw.body)+len(data) > MaxResponseSize {
		lg.Error(fmt.Sprintf(string(constants.ErrOptimizerResponseSizeLimitExceeded), MaxResponseSize))
		return rcw.ResponseWriter.Write(data)
	}

	rcw.body = append(rcw.body, data...)
	return rcw.ResponseWriter.Write(data)
}
