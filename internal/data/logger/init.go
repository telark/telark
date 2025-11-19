package logger

import "sync"

var loggerCache sync.Map

func GetLogger(prefix string) *CustomLogger {
	if lg, ok := loggerCache.Load(prefix); ok {
		return lg.(*CustomLogger)
	}

	lg := NewCustomLogger(prefix)
	if actual, loaded := loggerCache.LoadOrStore(prefix, lg); loaded {
		return actual.(*CustomLogger)
	}

	return lg
}
