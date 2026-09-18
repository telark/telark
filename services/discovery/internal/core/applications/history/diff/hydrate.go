package diff

import (
	"context"
	"math"
	"strconv"
	"strings"

	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/config"
	"github.com/telark/discovery/internal/constants"
	appshared "github.com/telark/discovery/internal/core/applications/shared"
	"github.com/telark/kcore/resources/workload"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const k8sFieldName = "name"

// Derived fields and workload baselines come from the generation-1 snapshot, not the live API:
// empty fields would read as "field added" at generation 2, and a live baseline read races the
// change that just happened. Called only when stored == nil (new application).
func HydrateApplicationFromV1Snapshots(
	ctx context.Context,
	app *application.Application,
	snaps []application.ApplicationSnapshot,
	getSnapshotManifest func(
		ctx context.Context,
		snapshotID string,
		scope string,
		namespace string,
		generation int,
	) ([]unstructured.Unstructured, error),
) {
	if app == nil || getSnapshotManifest == nil {
		return
	}
	agg := newHydrateAgg()
	scope := config.DefaultSnapshotScope()
	for i := range snaps {
		s := snaps[i]
		if s.Generation != constants.DefaultAddValue {
			continue
		}
		id := strings.TrimSpace(s.ID)
		ns := strings.TrimSpace(s.Namespace)
		if id == constants.EmptyString || ns == constants.EmptyString {
			continue
		}
		objs, err := getSnapshotManifest(ctx, id, scope, ns, s.Generation)
		if err != nil || len(objs) == constants.DefaultInitValue {
			continue
		}
		for j := range objs {
			processHydrateObject(&objs[j], ns, agg)
		}
	}
	applyHydrateAggToApp(app, agg)
}

type hydrateAgg struct {
	imagesSet       map[string]bool
	portsSet        map[int]bool
	envKeysSet      map[string]bool
	configMapSet    map[string]bool
	secretSet       map[string]bool
	serviceMappings map[string]bool
	ingressRules    map[string]bool
	baselines       map[hydrateWLKey]application.MetricsBaseline
}

type hydrateWLKey struct{ ns, kind, name string }

func newHydrateAgg() *hydrateAgg {
	return &hydrateAgg{
		imagesSet:       make(map[string]bool),
		portsSet:        make(map[int]bool),
		envKeysSet:      make(map[string]bool),
		configMapSet:    make(map[string]bool),
		secretSet:       make(map[string]bool),
		serviceMappings: make(map[string]bool),
		ingressRules:    make(map[string]bool),
		baselines:       make(map[hydrateWLKey]application.MetricsBaseline),
	}
}

func processHydrateObject(u *unstructured.Unstructured, ns string, agg *hydrateAgg) {
	switch u.GetKind() {
	case appshared.KindService:
		extractServiceMappingsUnstructured(u, agg.serviceMappings)
		return
	case appshared.KindIngress:
		extractIngressRulesUnstructured(u, agg.ingressRules)
		return
	}
	extractFieldsFromUnstructured(u, agg.imagesSet, agg.portsSet, agg.envKeysSet, agg.configMapSet, agg.secretSet)
	if !isWorkloadKindForHydration(u.GetKind()) {
		return
	}
	bl := baselineFromWorkloadUnstructured(u)
	if bl.Replicas != constants.DefaultInitValue {
		agg.baselines[hydrateWLKey{ns, u.GetKind(), u.GetName()}] = bl
	}
}

func applyHydrateAggToApp(app *application.Application, agg *hydrateAgg) {
	if len(agg.imagesSet) > constants.DefaultInitValue && len(app.Images) == constants.DefaultInitValue {
		app.Images = boolMapKeys(agg.imagesSet)
	}
	if len(agg.portsSet) > constants.DefaultInitValue && len(app.Ports) == constants.DefaultInitValue {
		app.Ports = intMapKeys(agg.portsSet)
	}
	if len(agg.envKeysSet) > constants.DefaultInitValue && len(app.EnvVarKeys) == constants.DefaultInitValue {
		app.EnvVarKeys = boolMapKeys(agg.envKeysSet)
	}
	if len(agg.configMapSet) > constants.DefaultInitValue && len(app.ConfigMapRefs) == constants.DefaultInitValue {
		app.ConfigMapRefs = boolMapKeys(agg.configMapSet)
	}
	if len(agg.secretSet) > constants.DefaultInitValue && len(app.SecretRefs) == constants.DefaultInitValue {
		app.SecretRefs = boolMapKeys(agg.secretSet)
	}
	applyNetworkFieldsToApp(app, agg)
	applyBaselinesToWorkloads(app, agg.baselines)
}

func applyNetworkFieldsToApp(app *application.Application, agg *hydrateAgg) {
	if len(agg.serviceMappings) > constants.DefaultInitValue && len(app.ServiceMappings) == constants.DefaultInitValue {
		app.ServiceMappings = boolMapKeys(agg.serviceMappings)
	}
	if len(agg.ingressRules) > constants.DefaultInitValue && len(app.IngressRules) == constants.DefaultInitValue {
		app.IngressRules = boolMapKeys(agg.ingressRules)
	}
}

func applyBaselinesToWorkloads(app *application.Application, baselines map[hydrateWLKey]application.MetricsBaseline) {
	if len(baselines) == constants.DefaultInitValue {
		return
	}
	for i := range app.Metrics.Workloads {
		w := &app.Metrics.Workloads[i]
		k := hydrateWLKey{w.Namespace, w.ResourceKind, w.ResourceName}
		if bl, ok := baselines[k]; ok {
			w.Baseline = bl
		}
	}
	if len(app.Metrics.Workloads) == constants.DefaultInitValue {
		app.Metrics.Workloads = seedWorkloadUsageFromBaselines(app, baselines)
	}
}

func isWorkloadKindForHydration(kind string) bool {
	switch kind {
	case appshared.KindDeployment, appshared.KindStatefulSet, appshared.KindDaemonSet:
		return true
	default:
		return false
	}
}

func extractFieldsFromUnstructured(
	u *unstructured.Unstructured,
	imagesSet map[string]bool,
	portsSet map[int]bool,
	envKeysSet map[string]bool,
	configMapSet map[string]bool,
	secretSet map[string]bool,
) {
	containers, found, _ := unstructured.NestedSlice(
		u.Object,
		constants.K8sObjectFieldSpec,
		constants.K8sObjectFieldTemplate,
		constants.K8sObjectFieldSpec,
		constants.K8sObjectFieldContainers,
	)
	if !found {
		return
	}
	for _, raw := range containers {
		c, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if img, ok := c[constants.K8sObjectFieldImage].(string); ok && img != constants.EmptyString {
			imagesSet[strings.TrimSpace(img)] = true
		}
		extractContainerPortsUnstructured(c, portsSet)
		extractContainerEnvUnstructured(c, envKeysSet, configMapSet, secretSet)
	}
	extractVolumesUnstructured(u, configMapSet, secretSet)
}

func extractContainerPortsUnstructured(c map[string]any, portsSet map[int]bool) {
	portsRaw, ok := c["ports"].([]any)
	if !ok {
		return
	}
	for _, p := range portsRaw {
		pm, ok := p.(map[string]any)
		if !ok {
			continue
		}
		if port, ok := pm["containerPort"].(int64); ok && port > constants.ZeroInt64 {
			portsSet[int(port)] = true
		}
	}
}

func extractContainerEnvUnstructured(
	c map[string]any,
	envKeysSet map[string]bool,
	configMapSet map[string]bool,
	secretSet map[string]bool,
) {
	envRaw, ok := c["env"].([]any)
	if !ok {
		return
	}
	for _, e := range envRaw {
		em, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if key, ok := em[k8sFieldName].(string); ok && key != constants.EmptyString {
			envKeysSet[key] = true
		}
		extractEnvValueFromRefs(em, configMapSet, secretSet)
	}
}

func extractEnvValueFromRefs(em map[string]any, configMapSet, secretSet map[string]bool) {
	vf, ok := em["valueFrom"].(map[string]any)
	if !ok {
		return
	}
	if cmRef, ok := vf["configMapKeyRef"].(map[string]any); ok {
		if name, ok := cmRef[k8sFieldName].(string); ok && name != constants.EmptyString {
			configMapSet[name] = true
		}
	}
	if secRef, ok := vf["secretKeyRef"].(map[string]any); ok {
		if name, ok := secRef[k8sFieldName].(string); ok && name != constants.EmptyString {
			secretSet[name] = true
		}
	}
}

func extractVolumesUnstructured(u *unstructured.Unstructured, configMapSet, secretSet map[string]bool) {
	volumes, found, _ := unstructured.NestedSlice(
		u.Object,
		constants.K8sObjectFieldSpec,
		constants.K8sObjectFieldTemplate,
		constants.K8sObjectFieldSpec,
		"volumes",
	)
	if !found {
		return
	}
	for _, v := range volumes {
		vm, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if cm, ok := vm["configMap"].(map[string]any); ok {
			if name, ok := cm[k8sFieldName].(string); ok && name != constants.EmptyString {
				configMapSet[name] = true
			}
		}
		if sec, ok := vm["secret"].(map[string]any); ok {
			if name, ok := sec["secretName"].(string); ok && name != constants.EmptyString {
				secretSet[name] = true
			}
		}
	}
}

func extractServiceMappingsUnstructured(u *unstructured.Unstructured, mappingSet map[string]bool) {
	svcType, _, _ := unstructured.NestedString(u.Object, constants.K8sObjectFieldSpec, "type")
	ports, found, _ := unstructured.NestedSlice(u.Object, constants.K8sObjectFieldSpec, "ports")
	if !found {
		return
	}
	for _, raw := range ports {
		p, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		protocol, prOK := p["protocol"].(string)
		port, ptOK := p["port"].(int64)
		if !prOK || !ptOK || port == constants.ZeroInt64 {
			continue
		}
		targetPort := svcTargetPortString(p["targetPort"])
		entry := svcType + constants.ColonSeparator + protocol + constants.ColonSeparator +
			strconv.Itoa(int(port)) + constants.ColonSeparator + strings.TrimSpace(targetPort)
		mappingSet[entry] = true
	}
}

func svcTargetPortString(raw any) string {
	switch v := raw.(type) {
	case string:
		return v
	case int64:
		return strconv.Itoa(int(v))
	default:
		return constants.EmptyString
	}
}

func extractIngressRulesUnstructured(u *unstructured.Unstructured, ruleSet map[string]bool) {
	rules, found, _ := unstructured.NestedSlice(u.Object, constants.K8sObjectFieldSpec, "rules")
	if !found {
		return
	}
	for _, raw := range rules {
		r, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		host := constants.EmptyString
		if v, ok := r["host"].(string); ok {
			host = strings.TrimSpace(v)
		}
		http, hasHTTP := r["http"].(map[string]any)
		if !hasHTTP {
			if host != constants.EmptyString {
				ruleSet[host] = true
			}
			continue
		}
		if paths, ok := http["paths"].([]any); ok {
			extractIngressPathsUnstructured(host, paths, ruleSet)
		}
	}
}

func extractIngressPathsUnstructured(host string, paths []any, ruleSet map[string]bool) {
	for _, raw := range paths {
		pm, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		path, pOK := pm["path"].(string)
		backend, bOK := pm["backend"].(map[string]any)
		svc, sOK := backend["service"].(map[string]any)
		svcName, nOK := svc[k8sFieldName].(string)
		if !pOK || !bOK || !sOK || !nOK {
			continue
		}
		ruleSet[host+constants.ColonSeparator+path+constants.ColonSeparator+strings.TrimSpace(svcName)] = true
	}
}

func baselineFromWorkloadUnstructured(u *unstructured.Unstructured) application.MetricsBaseline {
	raw := replicaCountFromWorkload(u)
	replicas := clampToInt32(raw)
	images := imagesFromPodTemplate(u)
	var firstImage string
	if len(images) > constants.DefaultInitValue {
		firstImage = images[constants.DefaultInitValue]
	}
	req, lim := resourceRequestsFromUnstructured(u)
	fingerprint := workload.ComputeBaselineFingerprint(firstImage, replicas, req.CPU, req.Memory, lim.CPU, lim.Memory)
	return application.MetricsBaseline{
		Fingerprint: fingerprint,
		Replicas:    replicas,
		Requests:    req,
		Limits:      lim,
	}
}

func clampToInt32(n int64) int32 {
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	if n < math.MinInt32 {
		return math.MinInt32
	}
	return int32(n)
}

func resourceRequestsFromUnstructured(u *unstructured.Unstructured) (req, lim application.ResourceValues) {
	containers, found, _ := unstructured.NestedSlice(
		u.Object,
		constants.K8sObjectFieldSpec,
		constants.K8sObjectFieldTemplate,
		constants.K8sObjectFieldSpec,
		constants.K8sObjectFieldContainers,
	)
	if !found || len(containers) == constants.DefaultInitValue {
		return req, lim
	}
	c, ok := containers[constants.DefaultInitValue].(map[string]any)
	if !ok {
		return req, lim
	}
	resources, ok := c["resources"].(map[string]any)
	if !ok {
		return req, lim
	}
	if requests, ok := resources["requests"].(map[string]any); ok {
		if v, ok := requests["cpu"].(string); ok {
			req.CPU = v
		}
		if v, ok := requests["memory"].(string); ok {
			req.Memory = v
		}
	}
	if limits, ok := resources["limits"].(map[string]any); ok {
		if v, ok := limits["cpu"].(string); ok {
			lim.CPU = v
		}
		if v, ok := limits["memory"].(string); ok {
			lim.Memory = v
		}
	}
	return req, lim
}

func seedWorkloadUsageFromBaselines(
	app *application.Application,
	baselines map[hydrateWLKey]application.MetricsBaseline,
) []application.WorkloadUsage {
	out := make([]application.WorkloadUsage, constants.DefaultInitValue, len(baselines))
	for i := range app.Resources {
		r := app.Resources[i]
		if !isWorkloadKindForHydration(r.Kind) {
			continue
		}
		k := hydrateWLKey{r.Namespace, r.Kind, r.Name}
		bl, ok := baselines[k]
		if !ok {
			continue
		}
		out = append(out, application.WorkloadUsage{
			ResourceName: r.Name,
			ResourceKind: r.Kind,
			Namespace:    r.Namespace,
			Baseline:     bl,
		})
	}
	return out
}

func boolMapKeys(m map[string]bool) []string {
	out := make([]string, constants.DefaultInitValue, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func intMapKeys(m map[int]bool) []int {
	out := make([]int, constants.DefaultInitValue, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
