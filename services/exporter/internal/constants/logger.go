package constants

import "github.com/telark/data/logger"

func GetLogger(prefix string) *logger.CustomLogger {
	return logger.GetLogger(prefix)
}
