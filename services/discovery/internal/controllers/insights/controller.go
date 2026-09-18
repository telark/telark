package insights

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	insightsdata "github.com/telark/data/insights"
	appresource "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/clients"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/discovery/shared"
	gcfgclient "github.com/telark/rest/clients/resources/globalconfig"
)

const (
	envTickInterval     = "INSIGHTS_TICK_INTERVAL_SEC"
	defaultTickInterval = 300

	kindDeployment    = "Deployment"
	kindStatefulSet   = "StatefulSet"
	kindDaemonSet     = "DaemonSet"
	kindJob           = "Job"
	kindCronJob       = "CronJob"
	workloadKindCount = 5
)

type Logger interface {
	Info(msg string)
	Warn(msg string)
	Error(msg string)
}

// Never waits on the model: enrichment fills the cache in the background and the UI reads it
// on its own interval.
type Controller struct {
	exporter   *clients.ExporterClient
	enrichment *clients.EnrichmentClient
	globalCfg  *gcfgclient.Client
	logger     Logger
	tick       time.Duration
}

func NewController(logger Logger) *Controller {
	return &Controller{
		exporter:   clients.NewExporterClient(),
		enrichment: clients.NewEnrichmentClient(),
		globalCfg:  gcfgclient.NewClient(),
		logger:     logger,
		tick:       resolveTickInterval(),
	}
}

func (c *Controller) Run(ctx context.Context) {
	t := time.NewTicker(c.tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.processOnce()
		}
	}
}

func (c *Controller) processOnce() {
	cfg, err := c.globalCfg.GetGlobalConfig()
	if err != nil {
		c.logger.Warn(fmt.Sprintf(string(constants.WarnInsightsGlobalConfigRead), err))
		return
	}
	// AI off or half-configured: nothing to dispatch. Cheapest possible tick.
	if !cfg.AI.Enabled || cfg.AI.Provider == constants.EmptyString {
		return
	}

	apps, err := c.exporter.GetAllApplications()
	if err != nil {
		c.logger.Warn(fmt.Sprintf(string(constants.WarnInsightsListApplications), err))
		return
	}
	if len(apps) == constants.DefaultInitValue {
		return
	}

	if err := c.enrichment.DispatchApplications(BuildSignals(apps)); err != nil {
		c.logger.Warn(fmt.Sprintf(string(constants.WarnInsightsDispatch), err))
		return
	}
	c.logger.Info(fmt.Sprintf(string(constants.InfoInsightsDispatched), len(apps)))
}

func BuildSignals(apps []*appresource.Application) []insightsdata.Signal {
	signals := make([]insightsdata.Signal, constants.DefaultInitValue, len(apps))
	for _, app := range apps {
		if app == nil {
			continue
		}
		rs := app.ResourceSummary
		signals = append(signals, insightsdata.Signal{
			Name:          app.Name,
			Namespace:     primaryNamespace(app),
			Images:        shared.DefaultSlice(app.Images, []string{}),
			Ports:         shared.DefaultSlice(app.Ports, []int{}),
			EnvVarKeys:    shared.DefaultSlice(app.EnvVarKeys, []string{}),
			ResourceKinds: dedupeKinds(app.Resources),
			HasIngress:    rs.Ingress > constants.DefaultInitValue,
			HasPVC:        rs.PersistentVolumeClaim > constants.DefaultInitValue,

			Replicas:      app.Health.TotalReplicas,
			ReadyReplicas: app.Health.ReadyReplicas,
			HealthStatus:  app.Health.Status,

			WorkloadKinds:    workloadKinds(rs),
			HasService:       rs.Service > constants.DefaultInitValue,
			HasHPA:           rs.HorizontalPodAutoscaler > constants.DefaultInitValue,
			HasNetworkPolicy: rs.NetworkPolicy > constants.DefaultInitValue,

			SecretRefs:    shared.DefaultSlice(app.SecretRefs, []string{}),
			ConfigMapRefs: shared.DefaultSlice(app.ConfigMapRefs, []string{}),

			ManagedBy: app.Managed.By,
			Chart:     derefString(app.Managed.Chart),

			ChangeVelocityPerDay: app.Metrics.Derived.ChangeVelocityPerDay,
			Incidents:            app.Metrics.Derived.TotalIncidents,
			Recoveries:           app.Metrics.Derived.TotalRecoveries,

			Workloads: buildWorkloads(app.Metrics.Workloads),
		})
	}
	return signals
}

func workloadKinds(rs appresource.ResourceSummary) []string {
	kinds := make([]string, constants.DefaultInitValue, workloadKindCount)
	if rs.Deployment > constants.DefaultInitValue {
		kinds = append(kinds, kindDeployment)
	}
	if rs.StatefulSet > constants.DefaultInitValue {
		kinds = append(kinds, kindStatefulSet)
	}
	if rs.DaemonSet > constants.DefaultInitValue {
		kinds = append(kinds, kindDaemonSet)
	}
	if rs.Job > constants.DefaultInitValue {
		kinds = append(kinds, kindJob)
	}
	if rs.CronJob > constants.DefaultInitValue {
		kinds = append(kinds, kindCronJob)
	}
	return kinds
}

func buildWorkloads(workloads []appresource.WorkloadUsage) []insightsdata.WorkloadSignal {
	out := make([]insightsdata.WorkloadSignal, constants.DefaultInitValue, len(workloads))
	for _, w := range workloads {
		limits := w.Baseline.Limits
		out = append(out, insightsdata.WorkloadSignal{
			Name:      w.ResourceName,
			Kind:      w.ResourceKind,
			Replicas:  int(w.Baseline.Replicas),
			QoS:       w.Usage.QoS,
			CPU:       w.Usage.Resources.TotalCPU,
			Memory:    w.Usage.Resources.TotalMemory,
			LimitsSet: limits.CPU != constants.EmptyString || limits.Memory != constants.EmptyString,
		})
	}
	return out
}

func derefString(s *string) string {
	if s == nil {
		return constants.EmptyString
	}
	return *s
}

func primaryNamespace(app *appresource.Application) string {
	if len(app.Namespaces.Items) > constants.DefaultInitValue {
		return app.Namespaces.Items[constants.DefaultInitValue].Name
	}
	return constants.EmptyString
}

func dedupeKinds(resources []appresource.Resource) []string {
	seen := make(map[string]bool, len(resources))
	out := make([]string, constants.DefaultInitValue, len(resources))
	for _, r := range resources {
		if !seen[r.Kind] {
			seen[r.Kind] = true
			out = append(out, r.Kind)
		}
	}
	return out
}

func resolveTickInterval() time.Duration {
	raw := os.Getenv(envTickInterval)
	if raw == constants.EmptyString {
		return time.Duration(defaultTickInterval) * time.Second
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= constants.DefaultInitValue {
		return time.Duration(defaultTickInterval) * time.Second
	}
	return time.Duration(parsed) * time.Second
}
