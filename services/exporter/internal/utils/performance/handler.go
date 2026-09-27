package performance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/telark/exporter/internal/constants"
	restconstants "github.com/telark/rest/constants"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

var (
	lg = constants.GetLogger(constants.PrefixOptimizer)
	// Built from the same GenericResponse the live path encodes, so a hit and a
	// render differ only in the message.
	cachedEnvelope = sync.OnceValues(func() (prefix, suffix []byte) {
		var buf bytes.Buffer
		_ = json.NewEncoder(&buf).Encode(response.NewGenericResponse(
			http.StatusOK, response.OperationSuccess, json.RawMessage(constants.CachedEnvelopeSentinel), constants.CachedResponse,
		))
		prefix, suffix, _ = bytes.Cut(buf.Bytes(), []byte(constants.CachedEnvelopeSentinel))
		return prefix, suffix
	})
)

func NewOptimizedHandler(handler http.HandlerFunc, timeout time.Duration) *OptimizedHandler {
	return &OptimizedHandler{handler: handler, timeout: timeout}
}

func NewDynamicOptimizedHandlerFunc(
	handler http.HandlerFunc,
	resourceType string,
	operation string,
) func(http.ResponseWriter, *http.Request) {
	timeout := GetTimeoutForResource(resourceType, operation)
	soh := NewOptimizedHandler(handler, timeout)
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
	if etag := clh.etag(cacheKey); etag != constants.EmptyString {
		w.Header().Set(constants.HeaderETag, etag)
		w.Header().Set(restconstants.HeaderCacheControl, restconstants.CacheControlNoCache)
		if r.Header.Get(constants.HeaderIfNoneMatch) == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	if clh.serveCached(w, requestID, cacheKey) {
		return
	}

	lg.Info(fmt.Sprintf(string(constants.InfOptimizerCacheMiss), requestID, cacheKey))
	clh.executeHandler(w, r, requestID, cacheKey)
}

func (clh *CachedListHandler) serveCached(w http.ResponseWriter, requestID string, cacheKey string) bool {
	blob, ok := clh.cachedBlob(cacheKey)
	if !ok {
		return false
	}
	lg.Info(fmt.Sprintf(string(constants.InfOptimizerCacheHit), requestID, cacheKey))
	writeCachedEnvelope(w, blob)
	return true
}

// The blob is the data field's JSON, validated when it was stored; it goes out
// spliced into the envelope as-is, so a hit is one write of the bytes and never
// a decode, re-encode or scan of a multi-megabyte body.
func (clh *CachedListHandler) cachedBlob(cacheKey string) ([]byte, bool) {
	if blob, ok := clh.localGet(cacheKey); ok {
		return blob, true
	}
	cached, exists := clh.optimizer.Get(cacheKey)
	if !exists {
		return nil, false
	}
	text, ok := cached.(string)
	if !ok {
		return nil, false
	}
	blob := []byte(text)
	clh.localPut(cacheKey, blob)
	return blob, true
}

func writeCachedEnvelope(w http.ResponseWriter, blob []byte) {
	prefix, suffix := cachedEnvelope()
	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w.Header().Set(constants.HeaderContentLength, strconv.Itoa(len(prefix)+len(blob)+len(suffix)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(prefix)
	_, _ = w.Write(blob)
	_, _ = w.Write(suffix)
}

// The key names the blob a client holds, so a poller still holding it is
// answered before the blob is read. A single-resource key has no generation
// and would never move. ponytail: a TTL rebuild under an unchanged generation
// keeps the validator; a per-blob validator beside the blob would close that.
func (clh *CachedListHandler) etag(cacheKey string) string {
	if clh.operation != constants.OpList || cacheKey == constants.EmptyString {
		return constants.EmptyString
	}
	return constants.WeakETagPrefix + strconv.Quote(cacheKey)
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
	if !clh.acquireRender(r.Context()) {
		lg.Warn(fmt.Sprintf(string(constants.WarnOptimizerRenderRefused), requestID, cacheKey))
		w.Header().Del(constants.HeaderETag)
		w.Header().Set(constants.HeaderRetryAfter, constants.ListRenderRetryAfter)
		responseutils.LogAndSendResponse(w, http.StatusServiceUnavailable, response.OperationUnavailable,
			string(constants.ErrOptimizerRenderBusy), nil, nil)
		return
	}
	defer clh.releaseRender()
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

// Waits ListRenderWait for a render slot: a request that would only queue
// behind the running renders is refused with Retry-After instead. Single
// resource reads are never bounded, so their handler has no slots.
func (clh *CachedListHandler) acquireRender(ctx context.Context) bool {
	if clh.renders == nil {
		return true
	}
	wait := time.NewTimer(constants.ListRenderWait)
	defer wait.Stop()
	select {
	case clh.renders <- struct{}{}:
		return true
	case <-wait.C:
		return false
	case <-ctx.Done():
		return false
	}
}

func (clh *CachedListHandler) releaseRender() {
	if clh.renders != nil {
		<-clh.renders
	}
}

func (clh *CachedListHandler) localGet(cacheKey string) ([]byte, bool) {
	clh.localMu.Lock()
	defer clh.localMu.Unlock()
	for i := range clh.local {
		if clh.local[i].key != cacheKey {
			continue
		}
		if time.Now().Before(clh.local[i].expires) {
			return clh.local[i].data, true
		}
		clh.local = slices.Delete(clh.local, i, i+constants.DefaultIncrementValue)
		return nil, false
	}
	return nil, false
}

func (clh *CachedListHandler) localPut(cacheKey string, blob []byte) {
	if clh.operation != constants.OpList || cacheKey == constants.EmptyString {
		return
	}
	clh.localMu.Lock()
	defer clh.localMu.Unlock()
	clh.local = slices.DeleteFunc(clh.local, func(b localBlob) bool { return b.key == cacheKey })
	if len(clh.local) >= constants.ListBlobLocalEntries {
		clh.local = slices.Delete(clh.local, constants.DefaultInitValue, constants.DefaultIncrementValue)
	}
	clh.local = append(clh.local, localBlob{key: cacheKey, data: blob, expires: time.Now().Add(clh.storeTTL())})
}

func (rcw *responseCaptureWriter) WriteHeader(statusCode int) {
	rcw.statusCode = statusCode
	if statusCode != http.StatusOK {
		rcw.Header().Del(constants.HeaderETag)
	}
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
