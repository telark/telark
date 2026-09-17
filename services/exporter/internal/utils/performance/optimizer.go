package performance

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/exporter/internal/cache"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	rediscache "github.com/telark/x-ware/redis/cache"
)

// Shares the pool initConnectivity already dialed with retry, so startup no
// longer opens a second pool and sleeps a cache interval when Redis is late.
func NewOptimizer(rdb *redis.Client) *Optimizer {
	return &Optimizer{cache: rediscache.NewCacheFromClient(rdb, constants.CacheTTL)}
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

func (o *Optimizer) SetTTL(key string, value any, ttl time.Duration) {
	if !cache.ValidateCacheKey(key) {
		return
	}
	_ = o.cache.Set(context.Background(), key, value, ttl)
}

func (o *Optimizer) Delete(key string) {
	if !cache.ValidateCacheKey(key) {
		return
	}
	_, _ = o.cache.Del(context.Background(), key)
}

// ListGeneration is the cache key segment for list responses. A pending bump
// deferred by the write window is applied here, by the reader, once the window
// has passed, so a list is never more than ListCacheMinAge behind the writes.
func (o *Optimizer) ListGeneration(resourceType string) string {
	ctx := context.Background()
	dirty, err := o.cache.Client.Exists(ctx, cache.ListGenerationDirtyKey(resourceType)).Result()
	if err == nil && dirty > int64(constants.DefaultInitValue) && o.openBumpWindow(ctx, resourceType) {
		o.bump(ctx, resourceType)
	}
	value, err := o.cache.Client.Get(ctx, cache.ListGenerationKey(resourceType)).Result()
	if err != nil {
		return constants.EmptyString
	}
	return value
}

// BumpListGeneration invalidates list responses after a write. Under a write
// storm (every discovery pass rewrites every app) each bump forced a full
// rebuild for the next reader; bumps are now coalesced into ListCacheMinAge
// windows and the deferred one is picked up by ListGeneration.
func (o *Optimizer) BumpListGeneration(resourceType string) {
	ctx := context.Background()
	if !o.openBumpWindow(ctx, resourceType) {
		if err := o.cache.Client.Set(ctx, cache.ListGenerationDirtyKey(resourceType),
			constants.DefaultIncrementValue, constants.ListCacheDirtyTTL).Err(); err != nil {
			lg.Error(fmt.Sprintf(string(constants.ErrCacheGenerationBumpFailed), resourceType, err))
		}
		return
	}
	o.bump(ctx, resourceType)
}

func (o *Optimizer) openBumpWindow(ctx context.Context, resourceType string) bool {
	ok, err := o.cache.Client.SetNX(ctx, cache.ListGenerationWindowKey(resourceType),
		constants.DefaultIncrementValue, constants.ListCacheMinAge).Result()
	return err == nil && ok
}

func (o *Optimizer) bump(ctx context.Context, resourceType string) {
	if err := o.cache.Client.Incr(ctx, cache.ListGenerationKey(resourceType)).Err(); err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrCacheGenerationBumpFailed), resourceType, err))
		return
	}
	_, _ = o.cache.Del(ctx, cache.ListGenerationDirtyKey(resourceType))
	// Every earlier generation's blobs are unreachable now; left to their TTL
	// they pile up at one full list per write (hundreds of copies under a burst).
	cursor := constants.CacheScanCursorEnd
	for {
		keys, next, err := o.cache.Scan(ctx, cursor, cache.ListKeyPattern(resourceType), constants.CacheScanCount)
		if err != nil {
			return
		}
		if len(keys) > constants.DefaultInitValue {
			_, _ = o.cache.Del(ctx, keys...)
		}
		if next == constants.CacheScanCursorEnd {
			return
		}
		cursor = next
	}
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
