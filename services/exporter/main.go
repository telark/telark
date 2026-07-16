package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	goredis "github.com/redis/go-redis/v9"
	dataerrors "github.com/telark/data/errors"
	datamessages "github.com/telark/data/messages"
	exporterauthz "github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/config"
	"github.com/telark/exporter/internal/constants"
	envmanager "github.com/telark/exporter/internal/managers/envs"
	exprdb "github.com/telark/exporter/internal/redis"
	"github.com/telark/exporter/internal/routes"
	"github.com/telark/exporter/internal/utils/async"
	"github.com/telark/exporter/internal/utils/performance"
	"github.com/telark/rest/connectivity"
	"github.com/telark/rest/router"
	xauthz "github.com/telark/x-ware/authz"
	"github.com/telark/x-ware/cors"
	rediscore "github.com/telark/x-ware/redis/core"
	redisinit "github.com/telark/x-ware/redis/init"
)

var lg = constants.GetLogger(constants.PrefixMain)

const snapshotsDirPerm = 0o755

func main() {
	config.ApplyKubernetesRESTRateLimit()
	initSnapshotsConfig()
	optimizer := initOptimizerWithRetry()
	initConnectivity()
	async.Init()

	authzMiddleware, err := initAuthz()
	if err != nil {
		lg.Error(fmt.Sprintf(string(dataerrors.ErrAuthzInitFailed), err))
		os.Exit(constants.ExitCodeFailure)
	}
	server := startServer(optimizer, authzMiddleware)
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

func startServer(optimizer *performance.Optimizer, authzMiddleware mux.MiddlewareFunc) *http.Server {
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

	go func() {
		lg.Info(string(constants.InfServerStarting))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			lg.Error(fmt.Sprintf(string(constants.ErrServerStartFailed), err))
		}
	}()
	return server
}

func initAuthz() (mux.MiddlewareFunc, error) {
	serviceToken, err := envmanager.InitServiceToken()
	if err != nil {
		return nil, err
	}

	middleware, err := xauthz.New(xauthz.Config{
		Resolver:     exporterauthz.NewResolver(),
		Requirements: exporterauthz.Requirements(),
		RouteKey:     router.KeyFromRequest,
		ServiceToken: serviceToken,
	})
	if err != nil {
		return nil, err
	}

	lg.Info(string(datamessages.SuccessAuthzEnabled))
	return middleware, nil
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
