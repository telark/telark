package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	goredis "github.com/redis/go-redis/v9"
	dataerrors "github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/messages"
	"github.com/telark/telark/internal/kcore/k8sclient"
	"github.com/telark/telark/internal/rest/connectivity"
	"github.com/telark/telark/internal/rest/router"
	restserver "github.com/telark/telark/internal/rest/server"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/internal/x-ware/cors"
	xwareredis "github.com/telark/telark/internal/x-ware/redis/stream"
	discoveryauthz "github.com/telark/telark/services/discovery/internal/authz"
	"github.com/telark/telark/services/discovery/internal/clients"
	"github.com/telark/telark/services/discovery/internal/config"
	"github.com/telark/telark/services/discovery/internal/constants"
	protectionctrl "github.com/telark/telark/services/discovery/internal/controllers/plans/protection"
	"github.com/telark/telark/services/discovery/internal/coordination"
	"github.com/telark/telark/services/discovery/internal/coordination/forcesync"
	"github.com/telark/telark/services/discovery/internal/coordination/leadergate"
	"github.com/telark/telark/services/discovery/internal/core/applications/insights"
	"github.com/telark/telark/services/discovery/internal/core/insightsindex"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection"
	"github.com/telark/telark/services/discovery/internal/discovery/listing"
	"github.com/telark/telark/services/discovery/internal/discovery/prewarm"
	"github.com/telark/telark/services/discovery/internal/handlers/cleanup/autoclean"
	insightshandler "github.com/telark/telark/services/discovery/internal/handlers/insights"
	protectionhandler "github.com/telark/telark/services/discovery/internal/handlers/plans/protection"
	applicationhandler "github.com/telark/telark/services/discovery/internal/handlers/resources/applications"
	"github.com/telark/telark/services/discovery/internal/handlers/rollback"
	"github.com/telark/telark/services/discovery/internal/helpers/async"
	redishelper "github.com/telark/telark/services/discovery/internal/helpers/redis"
	sharedhelper "github.com/telark/telark/services/discovery/internal/helpers/shared"
	tcfghelper "github.com/telark/telark/services/discovery/internal/helpers/telarkconfig"
	"github.com/telark/telark/services/discovery/internal/informers"
	"github.com/telark/telark/services/discovery/internal/routes"
	"github.com/telark/telark/services/discovery/internal/startup"
	"github.com/telark/telark/services/discovery/internal/state"
)

var (
	server          *http.Server
	lg              = constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	authzMiddleware func(http.Handler) http.Handler
)

var (
	serviceCtx    context.Context
	serviceCancel context.CancelFunc
	coordMu       sync.RWMutex
	coordBundle   *coordination.CoordinationBundle
	coordReplica  string
)

func main() {
	if msg := startup.ValidateRendererRegistry(); msg != constants.EmptyString {
		lg.Error(msg)
		os.Exit(constants.ExitCodeFatal)
	}
	// A misconfigured authz layer must never degrade into an open API, so this
	// stops the process rather than serving without it.
	mw, err := xauthz.NewFromEnv(discoveryauthz.NewResolver(), discoveryauthz.Requirements())
	if err != nil {
		lg.Error(fmt.Sprintf(string(dataerrors.ErrAuthzInitFailed), err))
		os.Exit(constants.ExitCodeFatal)
	}
	authzMiddleware = mw

	async.Init()
	supervisorCtx, supervisorCancel := context.WithCancel(context.Background())
	defer supervisorCancel()
	go startSupervisor(supervisorCtx)
	startMainService()

	<-getQuitChannel()
	lg.Info(string(constants.SuccessServiceShuttingDown))

	resignLeaderOnShutdown()
	async.Drain()
	cleanupConsumerOnShutdown()
	if serviceCancel != nil {
		serviceCancel()
	}

	quit := getQuitChannel()
	if quit == nil {
		lg.Error(string(constants.ErrQuitChannelNotAvailable))
		lg.Error(string(constants.ErrGracefulShutdownFailed))
		return
	}

	select {
	case quit <- syscall.SIGTERM:
		lg.Info(string(constants.SuccessShutdownSignalSent))
	default:
		lg.Error(string(constants.ErrFailedSendShutdownSignal))
		lg.Error(string(constants.ErrGracefulShutdownFailed))
	}
}

func startMainService() {
	serviceCtx, serviceCancel = context.WithCancel(context.Background())
	redishelper.ResetBootstrapReady()
	newRouter := router.NewRouter(routes.Routes)
	// Registered on the router: mux middleware runs after route matching, which
	// is what makes the matched path template available to find the rule.
	newRouter.Use(authzMiddleware)
	corsHandler := cors.NewCORS()
	handler := corsHandler(newRouter)

	server = &http.Server{
		Addr:              constants.ColonSeparator + state.HealthCheckPort,
		Handler:           handler,
		ReadHeaderTimeout: constants.DefaultReadHeaderTimeout,
		ReadTimeout:       constants.DefaultReadTimeout,
		WriteTimeout:      constants.DefaultWriteTimeout,
		IdleTimeout:       constants.DefaultIdleTimeout,
	}

	go startServerWithRecovery(server)
	go startBootstrapWithRecovery()
	startup.EnsureTelarkConfigReadyAsync(serviceCtx)
	startup.PatchClusterVersionAsync(serviceCtx)
	tcfghelper.StartExcludedNamespacesSync(serviceCtx)
}

func startServerWithRecovery(server *http.Server) {
	defer func() {
		if r := recover(); r != nil {
			lg.Info(fmt.Sprintf(string(constants.SuccessServerRecovered), r, debug.Stack()))
			time.Sleep(constants.PanicRecoveryDelay)
			go startServerWithRecovery(server)
		}
	}()

	restserver.ListenAndSignal(server, getQuitChannel, lg)
}

func startBootstrapWithRecovery() {
	config.ApplyKubernetesRESTRateLimit()
	redishelper.ResetBootstrapReady()
	defer func() {
		if r := recover(); r != nil {
			lg.Error(fmt.Sprintf(string(constants.ErrBootstrapPanicRecovered), r))
			time.Sleep(constants.PanicRecoveryDelay)
			go startBootstrapWithRecovery()
		}
	}()

	ctx := bootstrapContext()
	rdb := redishelper.NewRedisClientWithRetry(ctx)
	if rdb == nil {
		return
	}
	conn := initConnectivity(rdb)
	replicaID := os.Getenv(constants.EnvReplicaID)
	if strings.TrimSpace(replicaID) == constants.EmptyString {
		runStandaloneBootstrap(ctx, rdb, conn)
		return
	}
	startCoordinationBootstrap(ctx, rdb, conn, replicaID)
}

func startRollbackLeaderGatedIfEnabled(ctx context.Context) {
	_ = rollback.StartLeaderGated(ctx, leaderElectionForInformers)
}

func startAutoCleanupIfEnabled(ctx context.Context, rdb *goredis.Client) {
	cfg := config.LoadAutoCleanupConfig()
	if !cfg.Enabled {
		return
	}
	kubeClient, err := k8sclient.InitKubernetesClient()
	if err != nil || kubeClient == nil {
		lg.Error(fmt.Sprintf(string(constants.ErrRollbackKubeClientInitFailed), err))
		return
	}
	detector := autoclean.NewDetector(cfg, rdb, clients.NewExporterClient(), kubeClient, leaderElectionForInformers)
	go detector.Run(ctx)
}

func startProtectionPlanLeaderGated(ctx context.Context, rdb *goredis.Client) {
	kubeClient, err := k8sclient.InitKubernetesClient()
	if err != nil || kubeClient == nil {
		lg.Error(fmt.Sprintf(string(constants.ErrRollbackKubeClientInitFailed), err))
		return
	}
	svc, err := protection.BuildService(kubeClient, rdb, lg)
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrRollbackKubeClientInitFailed), err))
		return
	}
	protectionhandler.InitService(svc)
	ctrl := protectionctrl.NewController(svc, lg)
	leadergate.Start(ctx, ctrl, leaderElectionForInformers)
	interval, clamped := config.ReportCheckpointInterval()
	if clamped {
		lg.Warn(fmt.Sprintf(string(constants.WarnReportCheckpointClamped),
			constants.ReportCheckpointMinSec, constants.ReportCheckpointMaxSec))
	}
	lg.Info(fmt.Sprintf(string(constants.InfReportCheckpointInterval), interval))
	leadergate.Start(ctx, protectionctrl.NewCheckpointController(svc, lg, interval), leaderElectionForInformers)
}

func bootstrapContext() context.Context {
	if serviceCtx != nil {
		return serviceCtx
	}
	return context.Background()
}

func initConnectivity(rdb *goredis.Client) *connectivity.ConnectivityManager {
	conn := connectivity.New(rdb)
	connectivity.SetGlobal(conn)
	conn.Register(constants.ServiceIDDiscovery)
	return conn
}

func startDiscoveryWatchers(ctx context.Context, rdb *goredis.Client, replicaID string) {
	listing.InformersCache = informers.TryListResourcesInNamespaces
	listing.AppNamespacesCache = informers.AppNamespaces
	coordination.CancelCoalesceFn = informers.CancelAppCoalesce
	insights.Init(rdb, lg)
	startInsightsIndex(ctx, rdb)
	prewarm.PrewarmDiscovery(ctx, rdb)
	go informers.Run(ctx, informers.Config{
		RDB:          rdb,
		LeaderFn:     leaderElectionForInformers,
		ReplicaID:    replicaID,
		DiscoverApps: coordination.DiscoverApplications,
		RunPrewarmNS: coordination.RunPrewarmForNamespace,
	})
}

func startInsightsIndex(ctx context.Context, rdb *goredis.Client) {
	idx := insightsindex.New(insightsindex.Settings{
		Refresh:    config.InsightsIndexRefresh(),
		Resync:     config.InsightsIndexResync(),
		StaleAfter: config.InsightsStaleAfter(),
	})
	insightshandler.InitIndex(idx)
	go idx.Run(ctx, rdb, clients.NewProtectionPlanClient().List)
}

func runStandaloneBootstrap(ctx context.Context, rdb *goredis.Client, conn *connectivity.ConnectivityManager) {
	lg.Error(string(constants.ErrHostnameNotSet))
	startDiscoveryWatchers(ctx, rdb, constants.EmptyString)
	startRollbackLeaderGatedIfEnabled(ctx)
	startProtectionPlanLeaderGated(ctx, rdb)
	startAutoCleanupIfEnabled(ctx, rdb)
	conn.SetReady(constants.ServiceIDDiscovery, true)
	redishelper.SetBootstrapReady()
}

func startCoordinationBootstrap(
	ctx context.Context,
	rdb *goredis.Client,
	conn *connectivity.ConnectivityManager,
	replicaID string,
) {
	cfg := config.LoadCoordinationConfig()
	coord := coordination.NewCoordinationBundle(rdb, replicaID, cfg)

	err := coord.Stream.EnsureConsumerGroup(ctx, constants.StreamOperations, constants.ConsumerGroupName)
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrFailedEnsureConsumerGroup), err))
		startDiscoveryWatchers(ctx, rdb, replicaID)
		startRollbackLeaderGatedIfEnabled(ctx)
		startProtectionPlanLeaderGated(ctx, rdb)
		redishelper.SetBootstrapReady()
		return
	}

	coordMu.Lock()
	coordBundle = coord
	coordReplica = replicaID
	coordMu.Unlock()

	startDiscoveryWatchers(ctx, rdb, replicaID)
	go coordination.RunPrewarmLeaderLoop(ctx, coord, replicaID, rdb)
	go coordination.RunConsumerWorker(ctx, coord, replicaID, rdb)
	go runReplicaHeartbeat(ctx, rdb, replicaID)

	applicationhandler.SetCoordinationBundle(coord, replicaID)
	protectionhandler.SetCoordinationBundle(coord)
	startForceSyncSubsystem(ctx, coord, rdb, replicaID)
	startRollbackLeaderGatedIfEnabled(ctx)
	startProtectionPlanLeaderGated(ctx, rdb)
	startAutoCleanupIfEnabled(ctx, rdb)
	conn.SetReady(constants.ServiceIDDiscovery, true)
	redishelper.SetBootstrapReady()
}

func startForceSyncSubsystem(
	ctx context.Context,
	coord *coordination.CoordinationBundle,
	rdb *goredis.Client,
	replicaID string,
) {
	fsCfg := config.LoadForceSyncConfig()
	stream := forcesync.NewStreamOps(coord.Stream, fsCfg)
	if err := stream.EnsureGroup(ctx); err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrFailedEnsureConsumerGroup), err))
		return
	}
	dedup := forcesync.NewDedup(rdb, fsCfg.DedupTTL)
	exporter := clients.NewExporterClient()
	executor := func(jobCtx context.Context, replica, appName string) error {
		return coordination.RunForceSyncJob(jobCtx, coord, rdb, replica, appName)
	}
	manager := forcesync.NewManager(fsCfg, stream, dedup, exporter, executor, replicaID)
	maintenance := forcesync.NewMaintenance(fsCfg, stream)
	leaderLoop := forcesync.NewLeaderLoop(coord.Election, manager, maintenance, coord.Config)
	ingress := forcesync.NewIngress(stream, dedup, exporter)
	applicationhandler.SetForceSyncIngress(ingress)
	go leaderLoop.Run(ctx)
}

func leaderElectionForInformers(ctx context.Context) bool {
	coordMu.RLock()
	bundle := coordBundle
	coordMu.RUnlock()
	if bundle == nil {
		return true
	}
	ok, err := bundle.Election.IsLeader(ctx)
	return err == nil && ok
}

var getQuitChannel = restserver.SignalQuit()

func startSupervisor(ctx context.Context) {
	ticker := time.NewTicker(state.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !isServiceHealthy() {
				lg.Error(string(constants.ErrServiceHealthCheckFailed))
				restartService()
			}
		}
	}
}

func isServiceHealthy() bool {
	adr := sharedhelper.ConcatWithColon(state.HealthCheckHost, state.HealthCheckPort)
	conn, err := net.DialTimeout("tcp", adr, state.HealthCheckTimeout)
	if err != nil {
		return false
	}
	defer func() {
		if err := conn.Close(); err != nil {
			lg.Warn(fmt.Sprintf(string(constants.ErrFailedCloseHealthCheckConnection), err))
		}
	}()
	return true
}

var restartFailureCount = constants.DefaultInitValue

func restartService() {
	lg.Info(string(messages.SuccessStartingServer))

	if shouldSkipRestart() {
		return
	}

	shutdownExistingServices()
	time.Sleep(state.RestartDelay)
	startMainService()
}

func shouldSkipRestart() bool {
	restartFailureCount++
	if restartFailureCount > state.MaxRestartAttempts {
		lg.Error(string(constants.ErrTooManyRestartAttempts))
		return true
	}
	return false
}

func shutdownExistingServices() {
	shutdownHTTPServer()
	if serviceCancel != nil {
		serviceCancel()
	}
}

func shutdownHTTPServer() {
	if server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), state.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			lg.Error(fmt.Sprintf(string(constants.ErrFailedShutdownHTTPServer), err))
		}
	}
}

func resignLeaderOnShutdown() {
	coordMu.RLock()
	bundle := coordBundle
	coordMu.RUnlock()
	if bundle == nil || bundle.Election == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), bundle.Config.ElectionResignTimeout)
	defer cancel()
	if err := bundle.Election.Resign(ctx); err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrElectionResignFailed), err))
		return
	}
	lg.Info(string(constants.LogElectionResignedOnShutdown))
}

func cleanupConsumerOnShutdown() {
	coordMu.RLock()
	bundle := coordBundle
	replica := coordReplica
	coordMu.RUnlock()

	if bundle == nil || replica == constants.EmptyString {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), bundle.Config.ShutdownCleanupTimeout)
	defer cancel()

	reclaimAndRepublishPending(ctx, bundle, replica)

	err := bundle.Stream.DeleteConsumer(
		ctx,
		constants.StreamOperations,
		constants.ConsumerGroupName,
		replica,
	)
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrConsumerCleanupFailed), replica, err))
		return
	}
	lg.Info(fmt.Sprintf(string(constants.LogConsumerCleanedUp), replica))
}

func reclaimAndRepublishPending(
	ctx context.Context,
	coord *coordination.CoordinationBundle,
	replicaID string,
) {
	pending, err := coord.Stream.ClaimStale(
		ctx,
		constants.StreamOperations,
		constants.ConsumerGroupName,
		replicaID,
		constants.ZeroDuration,
		xwareredis.StreamStaleClaimMaxCount,
	)
	if err != nil || len(pending) == constants.DefaultInitValue {
		return
	}
	for _, msg := range pending {
		attempts := constants.DefaultInitValue
		if v, ok := msg.Values[constants.StreamMsgFieldAttempts]; ok {
			if s, ok := v.(string); ok {
				attempts, _ = strconv.Atoi(s)
			}
		}
		_, _ = coord.Stream.Publish(ctx, constants.StreamOperations, coordination.GenerateMsgPayload(
			coordination.MsgField(msg, constants.StreamMsgFieldAppName),
			coordination.MsgField(msg, constants.StreamMsgFieldNamespace),
			coordination.MsgField(msg, constants.StreamMsgFieldCycleID),
			coordination.MsgField(msg, constants.StreamMsgFieldOperation),
			coordination.MsgField(msg, constants.StreamMsgFieldEnqueuedAt),
			attempts+constants.DefaultAddValue,
		))
		_ = coord.Stream.Ack(ctx, constants.StreamOperations, constants.ConsumerGroupName, msg.ID)
	}
}

func runReplicaHeartbeat(ctx context.Context, rdb *goredis.Client, replicaID string) {
	addr := os.Getenv(constants.EnvPodIP)
	coordination.AdvertiseReplica(ctx, rdb, replicaID, addr)
	ticker := time.NewTicker(xwareredis.ReplicaHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			coordination.AdvertiseReplica(ctx, rdb, replicaID, addr)
		case <-ctx.Done():
			return
		}
	}
}
