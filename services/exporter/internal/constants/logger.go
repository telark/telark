package constants

import (
	"sync"

	"github.com/telark/data/logger"
)

var (
	loggerCache = make(map[string]*logger.CustomLogger)
	loggerMutex sync.RWMutex
)

func GetLogger(prefix string) *logger.CustomLogger {
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

	l := logger.NewCustomLogger(prefix)
	loggerCache[prefix] = l
	return l
}
