package server

import (
	"os"
	"os/signal"
	"syscall"
)

const signalChannelBuffer = 1

// SignalQuit creates the shutdown channel, registers it for SIGINT/SIGTERM, and
// returns an accessor usable as the QuitFunc for ListenAndSignal. Every service
// wired this same channel by hand; this owns it once.
func SignalQuit() QuitFunc {
	ch := make(chan os.Signal, signalChannelBuffer)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	return func() chan os.Signal { return ch }
}
