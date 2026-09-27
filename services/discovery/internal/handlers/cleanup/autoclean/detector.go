package autoclean

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	appresource "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/clients"
	"github.com/telark/discovery/internal/config"
	"github.com/telark/discovery/internal/constants"
	applicationhandler "github.com/telark/discovery/internal/handlers/resources/applications"
	"github.com/telark/discovery/internal/informers"
	"k8s.io/client-go/kubernetes"
)

type Detector struct {
	cfg      config.AutoCleanupConfig
	rdb      *redis.Client
	exporter *clients.ExporterClient
	kube     *kubernetes.Clientset
	leaderFn func(context.Context) bool
}

func NewDetector(
	cfg config.AutoCleanupConfig,
	rdb *redis.Client,
	exporter *clients.ExporterClient,
	kube *kubernetes.Clientset,
	leaderFn func(context.Context) bool,
) *Detector {
	return &Detector{
		cfg:      cfg,
		rdb:      rdb,
		exporter: exporter,
		kube:     kube,
		leaderFn: leaderFn,
	}
}

func (d *Detector) Run(ctx context.Context) {
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	lg.Info(fmt.Sprintf(string(constants.LogAutoCleanupDetectorStarted),
		int(d.cfg.CycleInterval/time.Second),
		d.cfg.EmptyCyclesRequired,
		int(d.cfg.GracePeriod/time.Second),
		d.cfg.DeleteEnabled,
	))

	t := time.NewTicker(d.cfg.CycleInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			d.runCycle(ctx)
		}
	}
}

func (d *Detector) runCycle(ctx context.Context) {
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	if d.leaderFn != nil && !d.leaderFn(ctx) {
		return
	}
	lg.Info(string(constants.LogAutoCleanupCycleStart))

	apps, err := d.exporter.GetAllApplications()
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrAutoCleanupListAppsFailed), err))
		return
	}
	for _, app := range apps {
		if app == nil || strings.TrimSpace(app.Name) == constants.EmptyString {
			continue
		}
		d.evaluateApp(ctx, app)
	}
}

func (d *Detector) evaluateApp(ctx context.Context, app *appresource.Application) {
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	name := app.Name

	if r := railResourcesEmpty(app); !r.pass && !namespacesGone(ctx, d.kube, app) && !informers.AppVanished(ctx, name) {
		d.handleNonEmpty(ctx, name, r)
		return
	}

	rails := []railResult{
		railNoActiveRollback(app),
		railNamespaceIncluded(ctx, app),
		railNamespaceExists(ctx, d.kube, app),
		railNoForceSync(ctx, d.rdb, name),
		railNoCoalesceBuffer(ctx, d.rdb, name),
		railNoGenerationLock(ctx, d.rdb, name),
		railNoAnalyzerInflight(ctx, d.rdb, name),
		railCleanupCooldown(ctx, d.rdb, name),
	}
	for _, r := range rails {
		if r.pass {
			continue
		}
		lg.Info(fmt.Sprintf(string(constants.LogAutoCleanupBlocked), name, r.name, r.reason))
		// Neither a rail error (Redis/K8s down) nor a soft block advances the streak.
		return
	}

	d.advanceStreakAndMaybeFire(ctx, app)
}

func (d *Detector) handleNonEmpty(ctx context.Context, appName string, r railResult) {
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	s, err := loadStreak(ctx, d.rdb, appName)
	if err == nil && s.Count > constants.DefaultInitValue {
		_ = clearStreak(ctx, d.rdb, appName)
		lg.Info(fmt.Sprintf(string(constants.LogAutoCleanupStreakReset), appName, r.name))
	}
}

func (d *Detector) advanceStreakAndMaybeFire(ctx context.Context, app *appresource.Application) {
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	name := app.Name

	s, err := loadStreak(ctx, d.rdb, name)
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrAutoCleanupStateWriteFailed), name, err))
		return
	}
	now := time.Now().UTC()
	if s.Count == constants.DefaultInitValue {
		s.FirstEmptyUnixMs = now.UnixMilli()
	}
	s.Count++
	streakTTL := d.cfg.CycleInterval*time.Duration(d.cfg.EmptyCyclesRequired+5) + d.cfg.GracePeriod
	if werr := saveStreak(ctx, d.rdb, name, s, streakTTL); werr != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrAutoCleanupStateWriteFailed), name, werr))
		return
	}

	firstSeen := time.UnixMilli(s.FirstEmptyUnixMs).UTC().Format(time.RFC3339)
	lg.Info(fmt.Sprintf(string(constants.LogAutoCleanupCandidate), name, s.Count, firstSeen))

	if r := railSustainedAbsence(s, d.cfg.EmptyCyclesRequired); !r.pass {
		lg.Info(fmt.Sprintf(string(constants.LogAutoCleanupBlocked), name, r.name, r.reason))
		return
	}
	if r := railGracePeriod(s, d.cfg.GracePeriod, now); !r.pass {
		lg.Info(fmt.Sprintf(string(constants.LogAutoCleanupBlocked), name, r.name, r.reason))
		return
	}

	d.fireCleanup(ctx, app, s, firstSeen)
}

func (d *Detector) fireCleanup(
	ctx context.Context,
	app *appresource.Application,
	s streakState,
	firstSeen string,
) {
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	name := app.Name
	nsNames := namespaceNames(app)
	snapCount := len(app.Snapshots)

	if !d.cfg.DeleteEnabled {
		lg.Info(fmt.Sprintf(string(constants.LogAutoCleanupDryRunWould),
			name, s.Count, firstSeen, snapCount, nsNames))
		return
	}

	inflight, ierr := isInflight(ctx, d.rdb, name)
	if ierr == nil && inflight {
		lg.Info(fmt.Sprintf(string(constants.LogAutoCleanupBlocked),
			name, "inflight", "auto-cleanup already running"))
		return
	}
	inflightTTL := constants.AppResetHandlerTimeout + constants.AutoCleanupInflightTTLBuffer
	_ = markInflight(ctx, d.rdb, name, inflightTTL)
	defer func() { _ = clearInflight(ctx, d.rdb, name) }()

	lg.Info(fmt.Sprintf(string(constants.LogAutoCleanupFiring),
		name, s.Count, firstSeen, snapCount, nsNames))
	start := time.Now().UTC()

	cctx, cancel := context.WithTimeout(ctx, constants.AppResetHandlerTimeout)
	defer cancel()
	// The streak stays so the next cycle retries instead of re-accumulating.
	if rerr := applicationhandler.RunReset(cctx, d.rdb, name); rerr != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrAutoCleanupResetFailed), name, rerr))
		return
	}

	_ = clearStreak(ctx, d.rdb, name)
	lg.Info(fmt.Sprintf(string(constants.LogAutoCleanupDone),
		name, time.Since(start).Milliseconds()))
}

func namespaceNames(app *appresource.Application) string {
	if app == nil || len(app.Namespaces.Items) == constants.DefaultInitValue {
		return constants.EmptyString
	}
	names := make([]string, constants.DefaultInitValue, len(app.Namespaces.Items))
	for i := range app.Namespaces.Items {
		names = append(names, app.Namespaces.Items[i].Name)
	}
	return strings.Join(names, ",")
}
