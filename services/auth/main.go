package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	goredis "github.com/redis/go-redis/v9"
	authauthz "github.com/telark/auth/internal/authz"
	"github.com/telark/auth/internal/cmd"
	"github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
	cleanupctrl "github.com/telark/auth/internal/controllers/cleanup"
	coordcleanup "github.com/telark/auth/internal/coordination/cleanup"
	cleanuphandler "github.com/telark/auth/internal/handlers/cleanup"
	authhelper "github.com/telark/auth/internal/helpers/auth"
	oidchelper "github.com/telark/auth/internal/helpers/oidc"
	redishelper "github.com/telark/auth/internal/helpers/redis"
	"github.com/telark/auth/internal/helpers/webauthn"
	"github.com/telark/auth/internal/routes"
	dataconstants "github.com/telark/data/constants"
	dataerrors "github.com/telark/data/errors"
	datamessages "github.com/telark/data/messages"
	"github.com/telark/rest/router"
	xauthz "github.com/telark/x-ware/authz"
	"github.com/telark/x-ware/cors"
)

var (
	server      *http.Server
	serverMutex sync.RWMutex
	quitChannel chan os.Signal
	quitMutex   sync.RWMutex
	lg          = constants.GetLogger(constants.LoggerPrefixAuthService)
)

func main() {
	if handled, code := cmd.Dispatch(os.Args); handled {
		os.Exit(code)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrFailedLoadConfig), err))
		os.Exit(constants.ExitCodeError)
	}

	bootstrapCfg, err := config.LoadBootstrapConfig()
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrFailedLoadConfig), err))
		os.Exit(constants.ExitCodeError)
	}
	lg.Info(fmt.Sprintf(string(constants.LogBootstrapConfig),
		bootstrapCfg.SelfRegistrationEnabled, bootstrapCfg.BootstrapAdmins))

	if err := webauthn.InitWebAuthn(&cfg.WebAuthn); err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrWebAuthnSetupFailed), err))
		os.Exit(constants.ExitCodeError)
	}

	rdb := redishelper.NewRedisClientWithRetry(context.Background())
	if rdb == nil {
		lg.Error(string(constants.ErrRedisClientUnavailable))
		os.Exit(constants.ExitCodeError)
	}
	lg.Info(string(constants.SuccessRedisConnected))
	authhelper.InitAsyncWorker()

	startCleanupSystem(rdb)

	quitMutex.Lock()
	quitChannel = make(chan os.Signal, constants.DefaultQuitChannelSize)
	quitMutex.Unlock()
	signal.Notify(quitChannel, syscall.SIGINT, syscall.SIGTERM)

	authzMiddleware, err := initAuthz()
	if err != nil {
		lg.Error(fmt.Sprintf(string(dataerrors.ErrAuthzInitFailed), err))
		os.Exit(constants.ExitCodeError)
	}

	startMainService(cfg, authzMiddleware)

	<-quitChannel
	lg.Info(string(constants.SuccessServiceShuttingDown))

	gracefulShutdown(cfg)
}

func initAuthz() (mux.MiddlewareFunc, error) {
	serviceToken := strings.TrimSpace(os.Getenv(dataconstants.EnvServiceToken))
	if serviceToken == constants.EmptyString {
		return nil, errors.New(string(dataerrors.ErrAuthzServiceTokenNotSet))
	}

	middleware, err := xauthz.New(xauthz.Config{
		Resolver:     authauthz.NewResolver(),
		Requirements: authauthz.Requirements(),
		RouteKey:     router.KeyFromRequest,
		ServiceToken: serviceToken,
	})
	if err != nil {
		return nil, err
	}

	lg.Info(string(datamessages.SuccessAuthzEnabled))
	return middleware, nil
}

func startMainService(cfg *config.Config, authzMiddleware mux.MiddlewareFunc) {
	newRouter := router.NewRouter(routes.Routes)

	// Registered on the router: mux middleware runs after route matching, which
	// is what makes the matched path template available to find the rule.
	newRouter.Use(authzMiddleware)

	corsHandler := cors.NewCORS()
	handler := corsHandler(newRouter)

	srv := &http.Server{
		Addr:              constants.ColonSeparator + cfg.Service.Port,
		Handler:           handler,
		ReadHeaderTimeout: cfg.Service.ReadHeaderTimeout,
		ReadTimeout:       cfg.Service.ReadTimeout,
		WriteTimeout:      cfg.Service.WriteTimeout,
		IdleTimeout:       cfg.Service.IdleTimeout,
	}

	serverMutex.Lock()
	server = srv
	serverMutex.Unlock()

	go startServerWithRecovery(srv, cfg.Service.Port)
}

func startServer(server *http.Server, port string) {
	lg.Info(fmt.Sprintf(string(constants.SuccessServiceStarted), port))

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
		}
	}
}

var panicRecoveryAttempts int

func startServerWithRecovery(server *http.Server, port string) {
	defer func() {
		if r := recover(); r != nil {
			panicRecoveryAttempts++
			if panicRecoveryAttempts >= constants.MaxPanicRecoveryAttempts {
				lg.Error(fmt.Sprintf(
					string(constants.ErrMaxPanicRecoveryAttemptsReached), constants.MaxPanicRecoveryAttempts))
				quit := getQuitChannel()
				if quit != nil {
					select {
					case quit <- syscall.SIGTERM:
					default:
					}
				}
				return
			}
			lg.Error(fmt.Sprintf(string(constants.ErrServerPanicRecovered), r, debug.Stack()))
			time.Sleep(constants.PanicRecoveryDelay)
			go startServerWithRecovery(server, port)
		}
	}()

	startServer(server, port)
}

func gracefulShutdown(cfg *config.Config) {
	quitMutex.RLock()
	quit := quitChannel
	quitMutex.RUnlock()

	if quit == nil {
		lg.Error(string(constants.ErrQuitChannelNotAvailable))
		lg.Error(string(constants.ErrGracefulShutdownFailed))
		return
	}

	serverMutex.RLock()
	srv := server
	serverMutex.RUnlock()

	if srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Service.ShutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			lg.Error(fmt.Sprintf(string(constants.ErrFailedShutdownHTTPServer), err))
		} else {
			lg.Info(string(constants.LogHTTPServerShutdownSuccess))
		}
	}

	oidchelper.StopJWKSRefresh()
	authhelper.DrainAsyncWorker()
	if cleanupSystemCancel != nil {
		cleanupSystemCancel()
	}
}

func getQuitChannel() chan os.Signal {
	quitMutex.RLock()
	defer quitMutex.RUnlock()
	return quitChannel
}

var cleanupSystemCancel context.CancelFunc

func startCleanupSystem(rdb *goredis.Client) {
	if rdb == nil {
		lg.Error(string(constants.ErrRedisClientUnavailable))
		return
	}
	cfg := config.LoadCleanupConfig()
	targets := cleanupctrl.DefaultTargets()
	reconciler := cleanupctrl.NewReconciler(cfg, targets, constants.GetLogger(constants.LoggerPrefixCleanup))

	//nolint:gosec // G118: cancel is stored in cleanupSystemCancel and invoked in gracefulShutdown.
	ctx, cancel := context.WithCancel(context.Background())
	cleanupSystemCancel = cancel

	system, err := coordcleanup.Bootstrap(ctx, rdb, cfg, reconciler, nil, replicaID(), cfg.ReconcileTick)
	if err != nil {
		lg.Error(fmt.Sprintf("[cleanup] bootstrap failed: %v", err))
		return
	}
	cleanuphandler.InitIngress(system.Ingress)
	go system.LeaderLoop.Run(ctx)
}

func replicaID() string {
	if id := os.Getenv(constants.EnvReplicaID); id != constants.EmptyString {
		return id
	}
	return constants.StandaloneReplicaID
}
