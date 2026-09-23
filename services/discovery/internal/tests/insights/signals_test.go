package insights

import (
	"slices"
	"testing"

	insightsdata "github.com/telark/data/insights"
	appresource "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/controllers/insights"
)

const (
	portHTTP       = 80
	portHTTPS      = 443
	changeVelocity = 1.5
	kindDeployment = "Deployment"
)

// The signal set is the data contract with the enrichment service: what is built
// here is what the model reasons over, and its cache key is derived from
// name+namespace. A drifted mapping silently produces insights for the wrong app
// or none at all, so the mapping is pinned.
func TestBuildSignalsMapsEveryField(t *testing.T) {
	signals := insights.BuildSignals([]*appresource.Application{signalsFixture(), nil})

	if len(signals) != constants.DefaultAddValue {
		t.Fatalf("got %d signals, want 1 (nil apps skipped)", len(signals))
	}
	s := signals[constants.DefaultInitValue]
	if s.Name != "shop" || s.Namespace != "prod" {
		t.Errorf("identity = %s/%s, want prod/shop", s.Namespace, s.Name)
	}
	assertSignalInventory(t, s)
	assertSignalPosture(t, s)
	if len(s.Workloads) != constants.DefaultAddValue {
		t.Fatalf("got %d workloads, want 1", len(s.Workloads))
	}
	assertWorkloadSignal(t, s.Workloads[constants.DefaultInitValue])
}

func signalsFixture() *appresource.Application {
	chart := "shop-1.0.0"
	return &appresource.Application{
		Name: "shop",
		Namespaces: appresource.Namespaces{
			Items: []appresource.NamespaceEntry{{Name: "prod"}, {Name: "staging"}},
		},
		Images:     []string{"nginx:1.27"},
		Ports:      []int{portHTTP, portHTTPS},
		EnvVarKeys: []string{"DB_HOST"},
		Resources: []appresource.Resource{
			{Kind: kindDeployment}, {Kind: "Service"}, {Kind: kindDeployment},
		},
		Health: appresource.Health{Status: "degraded", ReadyReplicas: constants.TwoValue, TotalReplicas: constants.ThreeValue},
		ResourceSummary: appresource.ResourceSummary{
			Ingress: constants.DefaultAddValue, PersistentVolumeClaim: constants.DefaultInitValue,
			Deployment: constants.DefaultAddValue, Service: constants.DefaultAddValue,
			NetworkPolicy: constants.DefaultAddValue, HorizontalPodAutoscaler: constants.DefaultAddValue,
		},
		SecretRefs:    []string{"db-secret"},
		ConfigMapRefs: []string{"app-config"},
		Managed:       appresource.Managed{By: "helm", Chart: &chart},
		Metrics: appresource.ApplicationMetrics{
			Derived: appresource.DerivedMetrics{
				ChangeVelocityPerDay: changeVelocity, TotalIncidents: constants.TwoValue, TotalRecoveries: constants.DefaultAddValue,
			},
			Workloads: []appresource.WorkloadUsage{{
				ResourceName: "shop-api",
				ResourceKind: kindDeployment,
				Baseline: appresource.MetricsBaseline{
					Replicas: constants.ThreeValue,
					Limits:   appresource.ResourceValues{CPU: "500m", Memory: "512Mi"},
				},
				Usage: appresource.Usage{
					QoS:       "Burstable",
					Resources: appresource.ResourceUsage{TotalCPU: "120m", TotalMemory: "256Mi"},
				},
			}},
		},
	}
}

func assertSignalInventory(t *testing.T, s insightsdata.Signal) {
	t.Helper()
	if !slices.Equal(s.Images, []string{"nginx:1.27"}) || !slices.Equal(s.Ports, []int{portHTTP, portHTTPS}) {
		t.Errorf("images/ports not carried: %+v", s)
	}
	if !slices.Equal(s.EnvVarKeys, []string{"DB_HOST"}) {
		t.Errorf("envVarKeys not carried: %v", s.EnvVarKeys)
	}
	if !slices.Equal(s.ResourceKinds, []string{kindDeployment, "Service"}) {
		t.Errorf("resourceKinds = %v, want deduped [Deployment Service]", s.ResourceKinds)
	}
	if !s.HasIngress || s.HasPVC {
		t.Errorf("flags = ingress:%v pvc:%v, want ingress:true pvc:false", s.HasIngress, s.HasPVC)
	}
	if s.Replicas != constants.ThreeValue || s.ReadyReplicas != constants.TwoValue || s.HealthStatus != "degraded" {
		t.Errorf("health = %d/%d %q, want 3/2 degraded", s.Replicas, s.ReadyReplicas, s.HealthStatus)
	}
}

func assertSignalPosture(t *testing.T, s insightsdata.Signal) {
	t.Helper()
	if !s.HasService || !s.HasHPA || !s.HasNetworkPolicy {
		t.Errorf("posture = svc:%v hpa:%v np:%v, want all true", s.HasService, s.HasHPA, s.HasNetworkPolicy)
	}
	if !slices.Equal(s.WorkloadKinds, []string{kindDeployment}) {
		t.Errorf("workloadKinds = %v, want [Deployment]", s.WorkloadKinds)
	}
	if !slices.Equal(s.SecretRefs, []string{"db-secret"}) || !slices.Equal(s.ConfigMapRefs, []string{"app-config"}) {
		t.Errorf("config refs not carried: secrets=%v configmaps=%v", s.SecretRefs, s.ConfigMapRefs)
	}
	if s.ManagedBy != "helm" || s.Chart != "shop-1.0.0" {
		t.Errorf("managed = %q %q, want helm shop-1.0.0", s.ManagedBy, s.Chart)
	}
	if s.ChangeVelocityPerDay != changeVelocity || s.Incidents != constants.TwoValue || s.Recoveries != constants.DefaultAddValue {
		t.Errorf("stability = %.1f/%d/%d, want 1.5/2/1", s.ChangeVelocityPerDay, s.Incidents, s.Recoveries)
	}
}

func assertWorkloadSignal(t *testing.T, w insightsdata.WorkloadSignal) {
	t.Helper()
	if w.Name != "shop-api" || w.Kind != kindDeployment || w.Replicas != constants.ThreeValue {
		t.Errorf("workload identity = %s/%s x%d, want shop-api/Deployment x3", w.Name, w.Kind, w.Replicas)
	}
	if w.QoS != "Burstable" || w.CPU != "120m" || w.Memory != "256Mi" || !w.LimitsSet {
		t.Errorf("workload usage = %q cpu=%q mem=%q limitsSet=%v, want Burstable 120m 256Mi true", w.QoS, w.CPU, w.Memory, w.LimitsSet)
	}
}

func TestBuildSignalsEmptySlicesStayEmptyNotNil(t *testing.T) {
	app := &appresource.Application{Name: "bare"}
	signals := insights.BuildSignals([]*appresource.Application{app})

	if len(signals) != constants.DefaultAddValue {
		t.Fatalf("got %d signals, want 1", len(signals))
	}
	s := signals[constants.DefaultInitValue]
	// nil slices marshal to JSON null; the Python side validates lists, so the
	// contract is empty lists, never null.
	if s.Images == nil || s.Ports == nil || s.EnvVarKeys == nil || s.ResourceKinds == nil {
		t.Errorf("nil slice leaked into the wire contract: %+v", s)
	}
	if s.WorkloadKinds == nil || s.SecretRefs == nil || s.ConfigMapRefs == nil || s.Workloads == nil {
		t.Errorf("nil slice leaked into the wire contract (new fields): %+v", s)
	}
	if s.Namespace != "" {
		t.Errorf("namespace = %q, want empty when app has none", s.Namespace)
	}
}
