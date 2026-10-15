package logger

import (
	"log"
	"os"
	"sync/atomic"
)

type CustomLogger struct {
	logger *log.Logger
}

var commandOutput atomic.Bool

func NewCustomLogger(prefix string) *CustomLogger {
	return &CustomLogger{
		logger: log.New(os.Stdout, prefix, log.Ldate|log.Ltime),
	}
}

// A command's stdout is its answer to the operator, so from here on every logger, cached or not,
// writes to stderr and drops its debug lines.
func UseCommandOutput() {
	commandOutput.Store(true)
}

func (c *CustomLogger) target() *log.Logger {
	if commandOutput.Load() {
		return log.New(os.Stderr, c.logger.Prefix(), c.logger.Flags())
	}
	return c.logger
}

func (c *CustomLogger) Info(message string) {
	c.target().Printf("[INFO] %s", message)
}

func (c *CustomLogger) Warn(message string) {
	c.target().Printf("[WARNING] %s", message)
}

func (c *CustomLogger) Debug(message string) {
	if commandOutput.Load() {
		return
	}
	c.logger.Printf("[DEBUG] %s", message)
}

func (c *CustomLogger) Error(message string) {
	c.target().Printf("[ERROR] %s", message)
}
