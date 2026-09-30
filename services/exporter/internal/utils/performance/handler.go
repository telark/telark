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

	if clh.serveCached(w, requestID, cacheKey) != nil {
		return
	}

	lg.Info(fmt.Sprintf(string(constants.InfOptimizerCacheMiss), requestID, cacheKey))
	clh.executeHandler(w, r, requestID, cacheKey)
}

func (clh *CachedListHandler) serveCached(w http.ResponseWriter, requestID string, cacheKey string) []byte {
	blob, ok := clh.cachedBlob(cacheKey)
	if ok {
		serveBlob(w, requestID, cacheKey, blob)
	}
	return blob
}

func serveBlob(w http.ResponseWriter, requestID string, cacheKey string, blob []byte) {
	lg.Info(fmt.Sprintf(string(constants.InfOptimizerCacheHit), requestID, cacheKey))
	writeCachedEnvelope(w, blob)
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

func (clh *CachedListHandler) joinFlight(cacheKey string) (*listFlight, bool) {
	clh.inflightMu.Lock()
	defer clh.inflightMu.Unlock()
	if flight, ok := clh.inflight[cacheKey]; ok {
		return flight, false
	}
	flight := &listFlight{done: make(chan struct{})}
	clh.inflight[cacheKey] = flight
	return flight, true
}

func (clh *CachedListHandler) inflightRelease(cacheKey string) {
	clh.inflightMu.Lock()
	defer clh.inflightMu.Unlock()
	delete(clh.inflight, cacheKey)
}

func (clh *CachedListHandler) executeHandler(w http.ResponseWriter, r *http.Request, requestID string, cacheKey string) {
	if cacheKey == constants.EmptyString {
		clh.render(w, r, requestID, cacheKey)
		return
	}
	// Ten concurrent misses on a 1 MB list must not mean ten rebuilds. A render
	// with nothing to share (an error, a refusal) hands the key to the next waiter.
	for {
		flight, leader := clh.joinFlight(cacheKey)
		if leader {
			clh.lead(w, r, requestID, cacheKey, flight)
			return
		}
		select {
		case <-flight.done:
		case <-r.Context().Done():
			refuseRender(w, requestID, cacheKey)
			return
		}
		if flight.blob != nil {
			serveBlob(w, requestID, cacheKey, flight.blob)
			return
		}
	}
}

// Deferred so a panicking render still frees its waiters; the entry goes before
// done closes, so a woken waiter never rejoins a finished flight.
func (clh *CachedListHandler) lead(w http.ResponseWriter, r *http.Request, requestID string, cacheKey string, flight *listFlight) {
	defer close(flight.done)
	defer clh.inflightRelease(cacheKey)
	if flight.blob = clh.serveCached(w, requestID, cacheKey); flight.blob == nil {
		flight.blob = clh.render(w, r, requestID, cacheKey)
	}
}

func (clh *CachedListHandler) render(w http.ResponseWriter, r *http.Request, requestID string, cacheKey string) []byte {
	if !clh.acquireRender(r.Context()) {
		refuseRender(w, requestID, cacheKey)
		return nil
	}
	defer clh.releaseRender()
	responseCapture := &responseCaptureWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           make([]byte, constants.DefaultInitValue),
	}

	clh.handler(responseCapture, r)

	if responseCapture.statusCode != http.StatusOK {
		lg.Info(fmt.Sprintf(string(constants.InfOptimizerCacheStoreSkipped), requestID, responseCapture.statusCode))
		return nil
	}
	return storeResponseInCache(clh, r, responseCapture, cacheKey, requestID)
}

func refuseRender(w http.ResponseWriter, requestID string, cacheKey string) {
	lg.Warn(fmt.Sprintf(string(constants.WarnOptimizerRenderRefused), requestID, cacheKey))
	w.Header().Del(constants.HeaderETag)
	w.Header().Set(constants.HeaderRetryAfter, constants.ListRenderRetryAfter)
	responseutils.LogAndSendResponse(w, http.StatusServiceUnavailable, response.OperationUnavailable,
		string(constants.ErrOptimizerRenderBusy), nil, nil)
}

// Waits for a slot until the request's deadline: a waiter holds only its key's
// flight, never a render buffer. Single resource reads have no slots.
func (clh *CachedListHandler) acquireRender(ctx context.Context) bool {
	if clh.renders == nil {
		return true
	}
	select {
	case clh.renders <- struct{}{}:
		// select picks at random when the deadline passed with a slot free.
		if ctx.Err() != nil {
			clh.releaseRender()
			return false
		}
		return true
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
	rcw.body = append(rcw.body, data...)
	return rcw.ResponseWriter.Write(data)
}
