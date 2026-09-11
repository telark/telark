package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/telark/data/logger"
	"github.com/telark/data/messages"
	"github.com/telark/notifier/internal/constants"
	"github.com/telark/notifier/internal/status"
	"github.com/telark/notifier/internal/subscribers/manager"
	"github.com/telark/rest/connectivity"
	restserver "github.com/telark/rest/server"
	rediscore "github.com/telark/x-ware/redis/core"
	redisinit "github.com/telark/x-ware/redis/init"
)

var getQuitChannel = restserver.SignalQuit()

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

	m := manager.NewManager()
	go func() {
		if err := manager.RetryStart(
			ctx,
			m.Start,
			constants.NatsStartRetrySeconds*time.Second,
			lg,
		); err != nil {
			return
		}
		if conn := connectivity.Global(); conn != nil {
			conn.SetReady(constants.ServiceID, true)
		}
	}()

	statusSrv := status.NewServer(m.IsConnected)
	go restserver.ListenAndSignal(statusSrv, getQuitChannel, lg)
	go logConnectionStatus(ctx, m, lg)

	select {
	case <-getQuitChannel():
		lg.Info(string(messages.SuccessReceivedShutdownSig))
	case <-ctx.Done():
		lg.Info(string(messages.SuccessContextCanceled))
	}

	shutdownStatusServer(statusSrv, lg)
	m.Shutdown()
	lg.Info(string(messages.SuccessOperation))
}

func logConnectionStatus(ctx context.Context, m *manager.Manager, lg *logger.CustomLogger) {
	ticker := time.NewTicker(constants.ConnectionLogIntervalSeconds * time.Second)
	defer ticker.Stop()

	connected := true
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if current := m.IsConnected(); current != connected {
				connected = current
				if !connected {
					lg.Warn(string(messages.SuccessNatsConnectionStatusDisconnected))
				}
			}
		}
	}
}

func shutdownStatusServer(srv *http.Server, lg *logger.CustomLogger) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.ProcessTimeoutSeconds*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrStatusServerShutdown), err))
	}
}
