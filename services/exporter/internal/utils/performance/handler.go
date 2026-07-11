package performance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/telark/exporter/internal/constants"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

var lg = constants.GetLogger(constants.PrefixOptimizer)

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

	cacheKey := clh.cacheKey(r)
	lg.Info(fmt.Sprintf(string(constants.InfOptimizerCacheKey), requestID, cacheKey, r.Method, r.URL.Path))

	if cached, exists := clh.optimizer.Get(cacheKey); exists {
		lg.Info(fmt.Sprintf(string(constants.InfOptimizerCacheHit), requestID, cacheKey))
		var cachedData any
		switch v := cached.(type) {
		case []byte:
			if err := json.Unmarshal(v, &cachedData); err != nil {
				lg.Error(fmt.Sprintf(string(constants.ErrOptimizerCacheParseError), requestID, err))
				clh.executeHandler(w, r, requestID, cacheKey)
				return
			}
		case string:
			if err := json.Unmarshal([]byte(v), &cachedData); err != nil {
				lg.Error(fmt.Sprintf(string(constants.ErrOptimizerCacheParseError), requestID, err))
				clh.executeHandler(w, r, requestID, cacheKey)
				return
			}
		default:
			cachedData = v
		}

		responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess, constants.CachedResponse, cachedData, nil)
		return
	}

	lg.Info(fmt.Sprintf(string(constants.InfOptimizerCacheMiss), requestID, cacheKey))
	clh.executeHandler(w, r, requestID, cacheKey)
}

func (clh *CachedListHandler) executeHandler(w http.ResponseWriter, r *http.Request, requestID string, cacheKey string) {
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
