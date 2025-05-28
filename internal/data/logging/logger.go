package logging

import (
	"log"
	"os"
)

type CustomLogger struct {
	logger *log.Logger
}

func NewCustomLogger(prefix string) *CustomLogger {
	return &CustomLogger{
		logger: log.New(os.Stdout, prefix, log.Ldate|log.Ltime),
	}
}

func (c *CustomLogger) Info(message string) {
	c.logger.Printf("[INFO] %s", message)
}

func (c *CustomLogger) Warning(message string) {
	c.logger.Printf("[WARNING] %s", message)
}

func (c *CustomLogger) Debug(message string) {
	c.logger.Printf("[DEBUG] %s", message)
}

func (c *CustomLogger) Error(message string) {
	c.logger.Printf("[ERROR] %s", message)
}
