package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	goredis "github.com/redis/go-redis/v9"
	dataerrors "github.com/telark/data/errors"
	exporterauthz "github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/config"
	"github.com/telark/exporter/internal/constants"
	snapshotexp "github.com/telark/exporter/internal/exporters/snapshot"
	"github.com/telark/exporter/internal/informers"
	envmanager "github.com/telark/exporter/internal/managers/envs"
	exprdb "github.com/telark/exporter/internal/redis"
	"github.com/telark/exporter/internal/routes"
	"github.com/telark/exporter/internal/startup"
	"github.com/telark/exporter/internal/utils/async"
	"github.com/telark/exporter/internal/utils/performance"
	snaputil "github.com/telark/exporter/internal/utils/snapshot"
	"github.com/telark/rest/connectivity"
	"github.com/telark/rest/router"
	restserver "github.com/telark/rest/server"
	xauthz "github.com/telark/x-ware/authz"
	"github.com/telark/x-ware/cors"
	rediscore "github.com/telark/x-ware/redis/core"
	redisinit "github.com/telark/x-ware/redis/init"
)

var (
	lg = constants.GetLogger(constants.PrefixMain)
	// Package level so a failed listener can signal shutdown from the goroutine
	// serving it, rather than leaving the process alive with no listener.
	getQuitChannel = restserver.SignalQuit()
)

const snapshotsDirPerm = 0o755

func main() {
	config.ApplyKubernetesRESTRateLimit()
	initSnapshotsConfig()
	lg.Info(fmt.Sprintf(string(constants.InfListRenderConcurrencyConfigured), envmanager.InitListRenderConcurrency()))
	optimizer := performance.NewOptimizer(initConnectivity())
	startup.SeedBuiltins()
	async.Init()
	gcCtx, stopGC := context.WithCancel(context.Background())
	go snapshotexp.StartSnapshotGC(gcCtx)
	go snaputil.StartStorageStatsRefresher(gcCtx)
	go informers.StartApplications(gcCtx)

	authzMiddleware, err := xauthz.NewFromEnv(exporterauthz.NewResolver(), exporterauthz.Requirements())
	if err != nil {
		lg.Error(fmt.Sprintf(string(dataerrors.ErrAuthzInitFailed), err))
		os.Exit(constants.ExitCodeFailure)
	}
	server := startServer(optimizer, authzMiddleware)
	waitForShutdown(server)
	stopGC()
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
	lg.Info(fmt.Sprintf(string(constants.InfSnapshotGCIntervalConfigured), envmanager.InitSnapshotGCInterval()))
	lg.Info(fmt.Sprintf(string(constants.InfSnapshotStatsRefreshConfigured), envmanager.InitSnapshotStatsRefreshInterval()))
	lg.Info(fmt.Sprintf(
		string(constants.InfSnapshotsPVCConfigured),
		envmanager.GetSnapshotsPVCNamespace(),
		envmanager.GetSnapshotsPVCName(),
	))
}

func initConnectivity() *goredis.Client {
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
		return nil
	}
	exprdb.Set(rdb)
	conn := connectivity.New(rdb)
	connectivity.SetGlobal(conn)
	conn.Register("exporter")
	conn.SetReady("exporter", true)
	return rdb
}

func startServer(optimizer *performance.Optimizer, authzMiddleware func(http.Handler) http.Handler) *http.Server {
	rt := router.NewRouter(routes.InitRoutes(optimizer))
	// Registered on the router rather than wrapped around it: mux middleware
	// runs after route matching, which is what makes the matched path template
	// available to derive the route's requirement.
	rt.Use(authzMiddleware)
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

	lg.Info(string(constants.InfServerStarting))
	go restserver.ListenAndSignal(server, getQuitChannel, lg)
	return server
}

func waitForShutdown(server *http.Server) {
	<-getQuitChannel()

	lg.Warn(string(constants.InfServerShuttingDown))
	ctx, cancel := context.WithTimeout(context.Background(), constants.ServerShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		lg.Error(fmt.Sprintf(string(constants.InfServerForcedShutdown), err))
	}
}
