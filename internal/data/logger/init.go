package logger

import "sync"

var (
	loggerCache = make(map[string]*CustomLogger)
	loggerMutex sync.RWMutex
)

func GetLogger(prefix string) *CustomLogger {
	loggerMutex.RLock()
	if lg, exists := loggerCache[prefix]; exists {
		loggerMutex.RUnlock()
		return lg
	}
	loggerMutex.RUnlock()

	loggerMutex.Lock()
	defer loggerMutex.Unlock()

	if lg, exists := loggerCache[prefix]; exists {
		return lg
	}

	lg := NewCustomLogger(prefix)
	loggerCache[prefix] = lg
	return lg
}
