package performance

import (
	"time"

	"github.com/telark/exporter/internal/constants"
)

type contextKey string

const (
	PoolSizeK8sClient     = 5
	PoolSizeDynamicClient = 5
	PoolSizeWorker        = 10
	QueueSizeWorker       = 100
	MaxConcurrentRequests = 50
	RequestIDLength       = 8
	RequestIDFormat       = "20060102150405"
	RequestIDKey          = "requestID"
	Charset               = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	MaxResponseSize       = 10 * 1024 * 1024 // 10MB limit
)

var RequestIDCtxKey = contextKey(RequestIDKey)

var DefaultTimeoutConfig = &TimeoutConfig{
	CreateTimeout: 30 * time.Second,
	GetTimeout:    15 * time.Second,
	ListTimeout:   20 * time.Second,
	UpdateTimeout: 45 * time.Second,
	PatchTimeout:  20 * time.Second,
	DeleteTimeout: 60 * time.Second,
	ResourceTimeouts: map[string]map[string]time.Duration{
		constants.ResourceApplication: {
			constants.OpCreate: 30 * time.Second,
			constants.OpGet:    15 * time.Second,
			constants.OpList:   20 * time.Second,
			constants.OpUpdate: 30 * time.Second,
			constants.OpPatch:  20 * time.Second,
			constants.OpDelete: 30 * time.Second,
		},
	},
}
