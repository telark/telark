package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/telark/exporter/internal/config"
	"github.com/telark/exporter/internal/constants"
	envmanager "github.com/telark/exporter/internal/managers/envs"
	exprdb "github.com/telark/exporter/internal/redis"
	"github.com/telark/exporter/internal/routes"
	"github.com/telark/exporter/internal/utils/async"
	"github.com/telark/exporter/internal/utils/performance"
	"github.com/telark/rest/connectivity"
	"github.com/telark/rest/router"
	"github.com/telark/x-ware/cors"
	rediscore "github.com/telark/x-ware/redis/core"
	redisinit "github.com/telark/x-ware/redis/init"
	goredis "github.com/redis/go-redis/v9"
)

var lg = constants.GetLogger(constants.PrefixMain)

const snapshotsDirPerm = 0o755

func main() {
	config.ApplyKubernetesRESTRateLimit()
	initSnapshotsConfig()
	optimizer := initOptimizerWithRetry()
	initConnectivity()
	async.Init()
	server := startServer(optimizer)
	waitForShutdown(server)
	async.Drain()
	optimizer.Close()
	lg.Warn(string(constants.InfServerExitedGracefully))
}

func initSnapshotsConfig() {
	snapshotsPath := envmanager.InitSnapshotsPath()
	lg.Info(fmt.Sprintf(string(constants.InfSnapshotsPathConfigured), snapshotsPath))
	for _, scope := range envmanager.GetSnapshotScopes() {
		if err := os.MkdirAll(envmanager.GetSnapshotsScopeRoot(scope.Name), snapshotsDirPerm); err != nil {
			lg.Error(fmt.Sprintf(string(constants.ErrSnapshotScopeRootCreateFailed), scope.Name, err))
		}
	}
	maxSnapVersions := envmanager.InitSnapshotsMaxVersions()
	lg.Info(fmt.Sprintf(string(constants.InfSnapshotsMaxVersionsConfigured), maxSnapVersions))
	lg.Info(fmt.Sprintf(
		string(constants.InfSnapshotsPVCConfigured),
		envmanager.GetSnapshotsPVCNamespace(),
		envmanager.GetSnapshotsPVCName(),
	))
}

func initConnectivity() {
	rdb := redisinit.NewClientWithRetry(
		context.Background(),
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
	if rdb == nil {
		return
	}
	exprdb.Set(rdb)
	conn := connectivity.New(rdb)
	connectivity.SetGlobal(conn)
	conn.Register("exporter")
	conn.SetReady("exporter", true)
}

func startServer(optimizer *performance.Optimizer) *http.Server {
	rt := router.NewRouter(routes.InitRoutes(optimizer))
	corsHandler := cors.NewCORS()
	handler := corsHandler(rt)

	server := &http.Server{
		Addr:           fmt.Sprintf(":%d", constants.DefaultPort),
		Handler:        handler,
		ReadTimeout:    constants.ServerReadTimeout,
		WriteTimeout:   constants.ServerWriteTimeout,
		IdleTimeout:    constants.ServerIdleTimeout,
		MaxHeaderBytes: constants.MaxHeaderBytes,
	}

	go func() {
		lg.Info(string(constants.InfServerStarting))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			lg.Error(fmt.Sprintf(string(constants.ErrServerStartFailed), err))
		}
	}()
	return server
}

func waitForShutdown(server *http.Server) {
	quit := make(chan os.Signal, constants.DefaultChannelBufferSize)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	lg.Warn(string(constants.InfServerShuttingDown))
	ctx, cancel := context.WithTimeout(context.Background(), constants.ServerShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		lg.Error(fmt.Sprintf(string(constants.InfServerForcedShutdown), err))
	}
}

func initOptimizerWithRetry() *performance.Optimizer {
	for {
		opt, err := performance.NewOptimizer()
		if err == nil {
			return opt
		}
		lg.Error(fmt.Sprintf(string(constants.ErrOptimizerInitFailed), err))
		lg.Warn(string(constants.InfOptimizerRetryingInitialization))
		time.Sleep(constants.CacheRefreshInterval)
	}
}
