package server

import (
	"os"
	"os/signal"
	"syscall"
)

const signalChannelBuffer = 1

func SignalQuit() QuitFunc {
	ch := make(chan os.Signal, signalChannelBuffer)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	return func() chan os.Signal { return ch }
}
