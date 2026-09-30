package performance

import (
	"net/http"
	"sync"
	"time"

	rediscache "github.com/telark/telark/internal/x-ware/redis/cache"
	"github.com/telark/telark/services/exporter/internal/constants"
	envmanager "github.com/telark/telark/services/exporter/internal/managers/envs"
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
	handler http.HandlerFunc
	timeout time.Duration
}

type CachedListHandler struct {
	optimizer    *Optimizer
	handler      http.HandlerFunc
	cacheKey     CacheKeyFunc
	resourceType string
	operation    string
	// One rebuild per cache key at a time; concurrent misses wait for its blob.
	inflightMu sync.Mutex
	inflight   map[string]*listFlight
	// Renders running at once across keys; nil leaves them unbounded.
	renders chan struct{}
	// Last rendered list blobs: a hit under the current generation is a memory
	// read, not a multi-megabyte Redis round trip.
	localMu sync.Mutex
	local   []localBlob
}

type listFlight struct {
	done chan struct{}
	blob []byte
}

type localBlob struct {
	key     string
	data    []byte
	expires time.Time
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
	clh := &CachedListHandler{
		optimizer:    optimizer,
		handler:      handler,
		cacheKey:     cacheKeyFunc,
		resourceType: resourceType,
		operation:    operation,
		inflight:     map[string]*listFlight{},
	}
	if operation == constants.OpList {
		clh.renders = make(chan struct{}, envmanager.GetListRenderConcurrency())
	}
	return clh
}
