package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/telark/data/messages"
	"github.com/telark/discovery/clients"
	"github.com/telark/discovery/config"
	"github.com/telark/discovery/constants"
	protectionctrl "github.com/telark/discovery/controllers/plans/protection"
	"github.com/telark/discovery/coordination"
	"github.com/telark/discovery/coordination/forcesync"
	"github.com/telark/discovery/core/plans/protection"
	"github.com/telark/discovery/discovery/listing"
	"github.com/telark/discovery/discovery/prewarm"
	"github.com/telark/discovery/handlers/cleanup/autoclean"
	protectionhandler "github.com/telark/discovery/handlers/plans/protection"
	applicationhandler "github.com/telark/discovery/handlers/resources/applications"
	"github.com/telark/discovery/handlers/rollback"
	"github.com/telark/discovery/helpers/async"
	gcfghelper "github.com/telark/discovery/helpers/globalconfig"
	redishelper "github.com/telark/discovery/helpers/redis"
	sharedhelper "github.com/telark/discovery/helpers/shared"
	"github.com/telark/discovery/informers"
	"github.com/telark/discovery/routes"
	"github.com/telark/discovery/startup"
	"github.com/telark/discovery/state"
	"github.com/telark/kcore/k8sclient"
	"github.com/telark/rest/connectivity"
	"github.com/telark/rest/router"
	"github.com/telark/x-ware/cors"
	xwareredis "github.com/telark/x-ware/redis/stream"
	goredis "github.com/redis/go-redis/v9"
)

var (
	server *http.Server
	lg     = constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
)

var (
	serviceCtx    context.Context
	serviceCancel context.CancelFunc
	coordMu       sync.RWMutex
	coordBundle   *coordination.CoordinationBundle
	coordReplica  string
)

func main() {
	quitChannel = make(chan os.Signal, constants.DefaultQuitChannelSize)
	signal.Notify(quitChannel, syscall.SIGINT, syscall.SIGTERM)

	if msg := startup.ValidateRendererRegistry(); msg != constants.EmptyString {
		lg.Error(msg)
		os.Exit(constants.ExitCodeFatal)
	}
	async.Init()
	supervisorCtx, supervisorCancel := context.WithCancel(context.Background())
	defer supervisorCancel()
	go startSupervisor(supervisorCtx)
	startMainService()

	<-quitChannel
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
	//nolint:gosec // G118: cancel is invoked via shutdownExistingServices on restart/shutdown.
	serviceCtx, serviceCancel = context.WithCancel(context.Background())
	redishelper.ResetBootstrapReady()
	newRouter := router.NewRouter(routes.Routes)
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
	startup.EnsureGlobalConfigReadyAsync(serviceCtx)
	startup.PatchClusterVersionAsync(serviceCtx)
	gcfghelper.StartExcludedNamespacesSync(serviceCtx)
}

func startServer(server *http.Server) {
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		lg.Error(fmt.Sprintf(string(constants.ErrServerFailedToStartDetail), err.Error()))
		lg.Error(string(constants.ErrServerInitiatingShutdown))

		quit := getQuitChannel()
		if quit == nil {
			lg.Error(string(constants.ErrQuitChannelNotAvailable))
			return
		}

		select {
		case quit <- syscall.SIGTERM:
			lg.Info(string(constants.SuccessShutdownSignalSent))
		default:
			lg.Error(string(constants.ErrFailedSendShutdownSignal))
			return
		}
	}
}

func startServerWithRecovery(server *http.Server) {
	defer func() {
		if r := recover(); r != nil {
			lg.Info(fmt.Sprintf(string(constants.SuccessServerRecovered), r, debug.Stack()))
			time.Sleep(constants.PanicRecoveryDelay)
			go startServerWithRecovery(server)
		}
	}()

	startServer(server)
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
	protectionctrl.StartLeaderGated(ctx, ctrl, leaderElectionForInformers)
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
	coordination.CancelCoalesceFn = informers.CancelAppCoalesce
	prewarm.PrewarmDiscovery(ctx, rdb)
	go informers.Run(ctx, informers.Config{
		RDB:          rdb,
		LeaderFn:     leaderElectionForInformers,
		ReplicaID:    replicaID,
		DiscoverApps: coordination.DiscoverApplications,
		RunPrewarmNS: coordination.RunPrewarmForNamespace,
	})
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
	maintenance := forcesync.NewMaintenance(fsCfg, stream, replicaID)
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

var quitChannel chan os.Signal

func getQuitChannel() chan os.Signal {
	return quitChannel
}

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
	waitForShutdown()
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

func waitForShutdown() {
	time.Sleep(state.RestartDelay)
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
	ticker := time.NewTicker(xwareredis.ReplicaHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			key := constants.KeyPrefixReplicaHB + replicaID + constants.KeySuffixReplicaHB
			rdb.Set(ctx, key, time.Now().UTC().String(), xwareredis.ReplicaHeartbeatTTL)
		case <-ctx.Done():
			return
		}
	}
}
