package concurrency

import "sync"

type LockManager struct {
	mu    sync.RWMutex
	locks map[string]*sync.Mutex
}

func NewLockManager() *LockManager {
	return &LockManager{locks: make(map[string]*sync.Mutex)}
}

var defaultManager = NewLockManager()

func GetLock(key string) *sync.Mutex {
	return defaultManager.getOrCreate(key)
}

func (m *LockManager) getOrCreate(key string) *sync.Mutex {
	if key == "" {
		return &sync.Mutex{}
	}
	m.mu.RLock()
	if l, ok := m.locks[key]; ok {
		m.mu.RUnlock()
		return l
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if l, ok := m.locks[key]; ok {
		return l
	}
	l := &sync.Mutex{}
	m.locks[key] = l
	return l
}
