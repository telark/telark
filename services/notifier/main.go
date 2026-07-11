package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/telark/data/errors"
	"github.com/telark/data/logger"
	"github.com/telark/data/messages"
	"github.com/telark/notifier/constants"
	"github.com/telark/notifier/subscribers/manager"
	"github.com/telark/rest/connectivity"
	rediscore "github.com/telark/x-ware/redis/core"
	redisinit "github.com/telark/x-ware/redis/init"
	goredis "github.com/redis/go-redis/v9"
)

func main() {
	lg := logger.GetLogger(constants.PrefixNotifierService)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rdb := redisinit.NewClientWithRetry(
		ctx,
		func() (*goredis.Client, error) {
			c, err := rediscore.InitClient()
			if err != nil {
				return nil, err
			}
			return c.Client, nil
		},
		redisinit.RetryConfig{},
		lg,
	)
	if rdb != nil {
		conn := connectivity.New(rdb)
		connectivity.SetGlobal(conn)
		conn.Register(constants.ServiceID)
	}

	sigChan := make(chan os.Signal, constants.SignalChanBuffer)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	m := manager.NewManager()
	if err := m.Start(); err != nil {
		lg.Error(fmt.Sprintf(string(errors.ErrNatsSubscriberManager), err))
	} else {
		if conn := connectivity.Global(); conn != nil {
			conn.SetReady(constants.ServiceID, true)
		}
	}

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if m.IsConnected() {
					lg.Info(string(messages.SuccessNatsConnectionStatusConnected))
				} else {
					lg.Info(string(messages.SuccessNatsConnectionStatusDisconnected))
				}
			}
		}
	}()

	select {
	case <-sigChan:
		lg.Info(string(messages.SuccessReceivedShutdownSig))
	case <-ctx.Done():
		lg.Info(string(messages.SuccessContextCanceled))
	}

	m.Shutdown()
	lg.Info(string(messages.SuccessOperation))
}
