package logger

import "sync"

var loggerCache sync.Map

func GetLogger(prefix string) *CustomLogger {
	if lg, ok := loggerCache.Load(prefix); ok {
		if customLogger, ok := lg.(*CustomLogger); ok {
			return customLogger
		}
	}

	lg := NewCustomLogger(prefix)
	if actual, loaded := loggerCache.LoadOrStore(prefix, lg); loaded {
		if customLogger, ok := actual.(*CustomLogger); ok {
			return customLogger
		}
	}

	return lg
}
