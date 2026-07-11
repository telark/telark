package constants

import (
	"sync"

	"github.com/telark/data/logger"
)

var loggerCache sync.Map

func GetLogger(prefix string) *logger.CustomLogger {
	if lg, ok := loggerCache.Load(prefix); ok {
		if typed, ok := lg.(*logger.CustomLogger); ok {
			return typed
		}
	}

	newLg := logger.NewCustomLogger(prefix)
	actual, _ := loggerCache.LoadOrStore(prefix, newLg)
	if typed, ok := actual.(*logger.CustomLogger); ok {
		return typed
	}
	return newLg
}
