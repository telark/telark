package server

import (
	"fmt"
	"net/http"
	"os"
	"syscall"
)

const (
	msgFailedToStart        = "server failed to start: %s"
	msgInitiatingShutdown   = "server initiating shutdown"
	msgQuitChannelMissing   = "quit channel not available, cannot signal shutdown"
	msgShutdownSignalSent   = "shutdown signal sent"
	msgShutdownSignalFailed = "failed to send shutdown signal"
)

type (
	QuitFunc func() chan os.Signal
	Logger   interface {
		Info(message string)
		Error(message string)
	}
)

// ListenAndSignal serves until the server stops, and turns a stop that is not a
// graceful close into a shutdown signal: a process whose listener died must not
// keep running and reporting itself healthy.
func ListenAndSignal(srv *http.Server, quit QuitFunc, log Logger) {
	err := srv.ListenAndServe()
	if err == nil || err == http.ErrServerClosed {
		return
	}

	log.Error(fmt.Sprintf(msgFailedToStart, err.Error()))
	log.Error(msgInitiatingShutdown)
	signalShutdown(quit, log)
}

func signalShutdown(quit QuitFunc, log Logger) {
	if quit == nil {
		log.Error(msgQuitChannelMissing)
		return
	}

	channel := quit()
	if channel == nil {
		log.Error(msgQuitChannelMissing)
		return
	}

	// Never block: a full channel means a shutdown is already under way.
	select {
	case channel <- syscall.SIGTERM:
		log.Info(msgShutdownSignalSent)
	default:
		log.Error(msgShutdownSignalFailed)
	}
}
