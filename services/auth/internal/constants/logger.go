package constants

import "github.com/telark/telark/internal/data/logger"

const (
	LoggerPrefixAuthService = "AuthService: "
	LoggerPrefixHandler     = "Handler: "
	LoggerPrefixHelper      = "Helper: "
	LoggerPrefixOIDC        = "OIDC: "
	LoggerPrefixRedis       = "Redis: "
	LoggerPrefixAsync       = "Async: "
	LoggerPrefixCleanup     = "Cleanup: "
)

func GetLogger(prefix string) *logger.CustomLogger {
	return logger.GetLogger(prefix)
}
