package constants

import (
	"sync"

	"github.com/telark/data/logger"
)

const (
	LoggerPrefixAuthService = "auth-service"
	LoggerPrefixHandler     = "handler"
	LoggerPrefixHelper      = "helper"
	LoggerPrefixWebAuthn    = "webauthn"
	LoggerPrefixConfig      = "config"
	LoggerPrefixOIDC        = "oidc"
	LoggerPrefixRedis       = "redis"
	LoggerPrefixAsync       = "async"
	LoggerPrefixCleanup     = "cleanup"
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
