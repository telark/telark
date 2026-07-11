package performance

import (
	"net/http"
	"time"

	rediscache "github.com/telark/x-ware/redis/cache"
)

type Optimizer struct {
	cache *rediscache.RedisCache
}

type TimeoutConfig struct {
	CreateTimeout    time.Duration
	GetTimeout       time.Duration
	ListTimeout      time.Duration
	UpdateTimeout    time.Duration
	PatchTimeout     time.Duration
	DeleteTimeout    time.Duration
	ResourceTimeouts map[string]map[string]time.Duration
}

type CacheKeyFunc func(r *http.Request) string

type OptimizedHandler struct {
	optimizer *Optimizer
	handler   http.HandlerFunc
	timeout   time.Duration
}

type CachedListHandler struct {
	optimizer    *Optimizer
	handler      http.HandlerFunc
	cacheKey     CacheKeyFunc
	resourceType string
	operation    string
}

type responseCaptureWriter struct {
	http.ResponseWriter
	statusCode int
	body       []byte
}

func NewCachedListHandler(
	optimizer *Optimizer,
	handler http.HandlerFunc,
	cacheKeyFunc CacheKeyFunc,
	resourceType string,
	operation string,
) *CachedListHandler {
	return &CachedListHandler{
		optimizer:    optimizer,
		handler:      handler,
		cacheKey:     cacheKeyFunc,
		resourceType: resourceType,
		operation:    operation,
	}
}
