package performance

import (
	"context"
	"net/http"

	"github.com/telark/exporter/cache"
	"github.com/telark/exporter/constants"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	rediscache "github.com/telark/x-ware/redis/cache"
	rediscore "github.com/telark/x-ware/redis/core"
)

func NewOptimizer() (*Optimizer, error) {
	manager := rediscore.NewRedisManager()
	redisCache, err := rediscache.NewCacheFromManager(manager, constants.CacheTTL)
	if err != nil {
		return nil, err
	}
	return &Optimizer{cache: redisCache}, nil
}

func (o *Optimizer) Get(key string) (any, bool) {
	if !cache.ValidateCacheKey(key) {
		return nil, false
	}
	val, err := o.cache.Get(context.Background(), key)
	if err != nil || val == "" {
		return nil, false
	}
	return val, true
}

func (o *Optimizer) Set(key string, value any) {
	if !cache.ValidateCacheKey(key) {
		return
	}
	_ = o.cache.Set(context.Background(), key, value, constants.DefaultInitValue)
}

func (o *Optimizer) Delete(key string) {
	if !cache.ValidateCacheKey(key) {
		return
	}
	_, _ = o.cache.Del(context.Background(), key)
}

func (o *Optimizer) Clear() {
	_ = o.cache.Flush(context.Background())
}

func (o *Optimizer) Close() {
	_ = o.cache.Close()
}

func CachedHandler(optimizer *Optimizer, handler http.HandlerFunc, keyFunc func(r *http.Request) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cacheKey := keyFunc(r)
		if cached, exists := optimizer.Get(cacheKey); exists {
			responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess, constants.CachedResponse, cached, nil)
			return
		}
		responseCapture := &responseCaptureWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			body:           make([]byte, constants.DefaultInitValue),
		}
		handler(responseCapture, r)
		if responseCapture.statusCode == http.StatusOK {
			optimizer.Set(cacheKey, responseCapture.body)
		}
	}
}

func KeyFunc(r *http.Request) string {
	return "cache:" + r.URL.Path + "?" + r.URL.RawQuery
}

func Handler(handler http.HandlerFunc) http.HandlerFunc {
	return handler
}
