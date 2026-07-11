package connectivity

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/rest/constants"
)

type ConnectivityManager struct {
	rdb *redis.Client

	mu       sync.RWMutex
	readyMap map[string]bool
	started  map[string]bool

	//nolint:containedctx
	ctx    context.Context
	cancel context.CancelFunc
}

var (
	globalMu sync.RWMutex
	globalV  *ConnectivityManager
)

func SetGlobal(m *ConnectivityManager) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalV = m
}

func Global() *ConnectivityManager {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalV
}

func New(rdb *redis.Client) *ConnectivityManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &ConnectivityManager{
		rdb:      rdb,
		readyMap: make(map[string]bool),
		started:  make(map[string]bool),
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (m *ConnectivityManager) Register(serviceID string) {
	if m == nil || serviceID == constants.EmptyString {
		return
	}
	m.mu.Lock()
	if _, ok := m.readyMap[serviceID]; !ok {
		m.readyMap[serviceID] = false
	}
	if m.started[serviceID] {
		m.mu.Unlock()
		return
	}
	m.started[serviceID] = true
	m.mu.Unlock()

	go m.syncLoop(serviceID)
}

func (m *ConnectivityManager) SetReady(serviceID string, ready bool) {
	if m == nil || serviceID == constants.EmptyString {
		return
	}
	m.mu.Lock()
	m.readyMap[serviceID] = ready
	m.mu.Unlock()
}

func (m *ConnectivityManager) IsReady(serviceID string) bool {
	if m == nil || serviceID == constants.EmptyString || m.rdb == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(constants.ConnectivityRedisOpTimeoutMillis)*time.Millisecond)
	defer cancel()
	val, err := m.rdb.Get(ctx, key(serviceID)).Result()
	if err != nil {
		return false
	}
	return val == constants.ConnectivityValueReady
}

func (m *ConnectivityManager) Close() {
	if m == nil {
		return
	}
	if m.cancel != nil {
		m.cancel()
	}
}

func (m *ConnectivityManager) syncLoop(serviceID string) {
	ticker := time.NewTicker(time.Duration(constants.ConnectivitySyncIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.syncOnce(serviceID)
		}
	}
}

func (m *ConnectivityManager) syncOnce(serviceID string) {
	if m == nil || serviceID == constants.EmptyString || m.rdb == nil {
		return
	}
	m.mu.RLock()
	ready := m.readyMap[serviceID]
	m.mu.RUnlock()

	val := constants.ConnectivityValueNotReady
	if ready {
		val = constants.ConnectivityValueReady
	}

	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(constants.ConnectivityRedisOpTimeoutMillis)*time.Millisecond)
	defer cancel()
	_ = m.rdb.Set(ctx, key(serviceID), val, time.Duration(constants.ConnectivityKeyTTLSeconds)*time.Second).Err()
}

func key(serviceID string) string {
	return fmt.Sprintf("%s%s", constants.ConnectivityKeyPrefix, serviceID)
}
