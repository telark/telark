package constants

import "github.com/telark/telark/internal/data/logger"

const (
	LoggerPrefixAuthService = "auth-service"
	LoggerPrefixHandler     = "handler"
	LoggerPrefixHelper      = "helper"
	LoggerPrefixOIDC        = "oidc"
	LoggerPrefixRedis       = "redis"
	LoggerPrefixAsync       = "async"
	LoggerPrefixCleanup     = "cleanup"
)

func GetLogger(prefix string) *logger.CustomLogger {
	return logger.GetLogger(prefix)
}
