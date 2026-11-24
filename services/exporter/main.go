package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	// The image is FROM scratch, and repeating plans name IANA zones: the zone data ships in the binary.
	_ "time/tzdata"

	goredis "github.com/redis/go-redis/v9"
	dataerrors "github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/rest/connectivity"
	"github.com/telark/telark/internal/rest/router"
	restserver "github.com/telark/telark/internal/rest/server"
	"github.com/telark/telark/internal/x-ware/async"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/internal/x-ware/cors"
	rediscore "github.com/telark/telark/internal/x-ware/redis/core"
	redisinit "github.com/telark/telark/internal/x-ware/redis/init"
	"github.com/telark/telark/services/exporter/internal/authrequests"
	exporterauthz "github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/bootstrap"
	"github.com/telark/telark/services/exporter/internal/config"
	"github.com/telark/telark/services/exporter/internal/constants"
	reportsexp "github.com/telark/telark/services/exporter/internal/exporters/reports"
	snapshotexp "github.com/telark/telark/services/exporter/internal/exporters/snapshot"
	"github.com/telark/telark/services/exporter/internal/informers"
	envmanager "github.com/telark/telark/services/exporter/internal/managers/envs"
	"github.com/telark/telark/services/exporter/internal/membership"
	"github.com/telark/telark/services/exporter/internal/migrations"
	exprdb "github.com/telark/telark/services/exporter/internal/redis"
	notifstorage "github.com/telark/telark/services/exporter/internal/redis/notifications"
	"github.com/telark/telark/services/exporter/internal/routes"
	"github.com/telark/telark/services/exporter/internal/startup"
	"github.com/telark/telark/services/exporter/internal/uninstall"
	"github.com/telark/telark/services/exporter/internal/utils/artifact"
	"github.com/telark/telark/services/exporter/internal/utils/performance"
	snaputil "github.com/telark/telark/services/exporter/internal/utils/snapshot"
)

var (
	lg = constants.GetLogger(constants.PrefixMain)
	// Package level so a failed listener can signal shutdown from the goroutine
	// serving it, rather than leaving the process alive with no listener.
	getQuitChannel = restserver.SignalQuit()
)

func main() {
	if code, ran := runSubcommand(os.Args[constants.DefaultIncrementValue:]); ran {
		os.Exit(code)
	}
	config.ApplyKubernetesRESTRateLimit()
	initSnapshotsConfig()
	initReportsConfig()
	lg.Info(fmt.Sprintf(string(constants.InfListRenderConcurrencyConfigured), envmanager.InitListRenderConcurrency()))
	envmanager.InitBootstrapAdmin()
	if err := errors.Join(envmanager.InitAuthRequestWait(), envmanager.InitAuthRequestRetention()); err != nil {
		lg.Error(err.Error())
		os.Exit(constants.ExitCodeFailure)
	}
	optimizer := performance.NewOptimizer(initConnectivity())
	startup.SeedTelarkConfig()
	// The record lives in TelarkConfig. The guard and the steps run before any other write, so an
	// older exporter rewrites nothing and each step sees the data as the previous release left it.
	if err := migrations.Run(migrations.Steps); err != nil {
		lg.Error(err.Error())
		os.Exit(constants.ExitCodeFailure)
	}
	// At every start rather than as a one-time step: a rollback to 0.1.x can leave a second marked account.
	if err := bootstrap.PinEnrolled(optimizer); err != nil {
		lg.Warn(fmt.Sprintf(string(constants.WarnBootstrapReconcileFailed), err))
	}
	startup.SeedBuiltins()
	async.Init()
	gcCtx, stopGC := context.WithCancel(context.Background())
	go snapshotexp.StartSnapshotGC(gcCtx)
	go reportsexp.StartReportsGC(gcCtx)
	go snaputil.StartStorageStatsRefresher(gcCtx)
	go informers.StartApplications(gcCtx)
	go informers.StartSessions(gcCtx)
	go informers.StartAuthRequests(gcCtx)
	go authrequests.StartCleanup(gcCtx)
	notifHub := notifstorage.NewHub(exprdb.Get())
	go notifHub.Run(gcCtx)
	go membership.Reconcile(gcCtx, optimizer)

	authzMiddleware, err := xauthz.NewFromEnv(exporterauthz.NewResolver(), exporterauthz.Requirements())
	if err != nil {
		lg.Error(fmt.Sprintf(string(dataerrors.ErrAuthzInitFailed), err))
		os.Exit(constants.ExitCodeFailure)
	}
	server := startServer(optimizer, notifHub, authzMiddleware)
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
		if err := os.MkdirAll(envmanager.GetSnapshotsScopeRoot(scope.Name), constants.SnapshotDirPerm); err != nil {
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

// The chart runs both: an init container when the exporter's claims move (templates/_storage.tpl),
// and the pre-delete hook once auth is stopped (templates/exporter/uninstall-finalizers.yaml).
func runSubcommand(args []string) (code int, ran bool) {
	if len(args) == constants.DefaultInitValue {
		return constants.DefaultInitValue, false
	}
	switch args[constants.DefaultInitValue] {
	case constants.SubcommandMigrateStorage:
		return migrateStorage(args[constants.DefaultIncrementValue:]), true
	case constants.SubcommandRemoveFinalizers:
		return removeFinalizers(), true
	}
	return constants.DefaultInitValue, false
}

func migrateStorage(pairs []string) int {
	if len(pairs) == constants.DefaultInitValue || len(pairs)%constants.MigrateStoragePairSize != constants.DefaultInitValue {
		lg.Error(string(constants.ErrMigrateStorageUsage))
		return constants.ExitCodeFailure
	}
	for pair := range slices.Chunk(pairs, constants.MigrateStoragePairSize) {
		from, to := pair[constants.DefaultInitValue], pair[constants.DefaultIncrementValue]
		copied, err := artifact.CopyTree(from, to)
		if err != nil {
			lg.Error(fmt.Sprintf(string(constants.ErrMigrateStorageFailed), from, to, err))
			return constants.ExitCodeFailure
		}
		lg.Info(fmt.Sprintf(string(constants.InfMigrateStorageCopied), copied, from, to))
	}
	return constants.DefaultInitValue
}

func removeFinalizers() int {
	config.ApplyKubernetesRESTRateLimit()
	result := uninstall.RemoveFinalizers(lg)
	lg.Info(fmt.Sprintf(string(constants.InfFinalizersRemoved), result.Removed, result.Failed))
	return constants.DefaultInitValue
}

func initReportsConfig() {
	reportsPath := envmanager.InitReportsPath()
	lg.Info(fmt.Sprintf(string(constants.InfReportsPathConfigured), reportsPath))
	plansRoot := filepath.Join(reportsPath, constants.ReportsPlansSubdir)
	if err := os.MkdirAll(plansRoot, constants.SnapshotDirPerm); err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrReportsRootCreateFailed), plansRoot, err))
	}
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

func startServer(
	optimizer *performance.Optimizer,
	notifHub *notifstorage.Hub,
	authzMiddleware func(http.Handler) http.Handler,
) *http.Server {
	rt := router.NewRouter(routes.InitRoutes(optimizer, notifHub))
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
	// Shutdown waits for open requests, and a notification stream never ends on its own.
	server.RegisterOnShutdown(notifHub.Close)

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
