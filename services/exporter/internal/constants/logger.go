package constants

import "github.com/telark/telark/internal/data/logger"

func GetLogger(prefix string) *logger.CustomLogger {
	return logger.GetLogger(prefix)
}
