package core

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/telark/discovery/internal/constants"
	apputils "github.com/telark/discovery/internal/core/applications/history/utils"
	appshared "github.com/telark/discovery/internal/core/applications/shared"
	"github.com/telark/discovery/internal/discovery/derivation"
	"github.com/telark/kcore/resources/autoscaling"
	"github.com/telark/kcore/resources/core"
	"github.com/telark/kcore/resources/networking"
	"github.com/telark/kcore/resources/workload"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

var GetManifestFromCache func(kind, name, ns string) (map[string]any, bool)

var envVarSkipUpper = func() []string {
	parts := strings.Split(strings.ToUpper(envKeySkipSubstr), csvSeparator)
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}()

func DiscoverInputsWithK8s(
	ctx context.Context,
	inputs []derivation.ResourceInput,
) []derivation.ResourceInput {
	if len(inputs) == constants.DefaultInitValue {
		return inputs
	}
	cache := buildEnrichCache(ctx, inputs)
	out := make([]derivation.ResourceInput, len(inputs))
	for i := range inputs {
		enrichOneInputFromCache(out, inputs, i, cache)
	}
	return out
}

func enrichOneInputFromCache(
	out []derivation.ResourceInput,
	inputs []derivation.ResourceInput,
	i int,
	cache map[string]enrichResult,
) {
	out[i] = inputs[i]
	ensureResourceInputSlices(&out[i])
	key := inputs[i].Namespace + keySeparator + inputs[i].Kind + keySeparator + inputs[i].Name
	if res, ok := cache[key]; ok {
		applyEnrichResult(&out[i], res)
	}
}

func ensureResourceInputSlices(in *derivation.ResourceInput) {
	if in.Images == nil {
		in.Images = []string{}
	}
	if in.Ports == nil {
		in.Ports = []int{}
	}
	if in.EnvVarKeys == nil {
		in.EnvVarKeys = []string{}
	}
	if in.ConfigMapRefs == nil {
		in.ConfigMapRefs = []string{}
	}
	if in.SecretRefs == nil {
		in.SecretRefs = []string{}
	}
	if in.ServiceMappings == nil {
		in.ServiceMappings = []string{}
	}
	if in.IngressRules == nil {
		in.IngressRules = []string{}
	}
}

func applyEnrichResult(in *derivation.ResourceInput, res enrichResult) {
	if !res.createdAt.IsZero() {
		in.CreatedAt = res.createdAt
	}
	if res.lastModifiedBy != constants.EmptyString {
		in.LastModifiedBy = res.lastModifiedBy
	}
	if !res.lastModifiedAt.IsZero() {
		in.LastModifiedAt = res.lastModifiedAt
	}
	if res.lastModifiedOp != constants.EmptyString {
		in.LastModifiedOp = res.lastModifiedOp
	}
	if res.spec != nil {
		in.Images = extractImages(res.spec)
		in.Ports = extractPorts(res.spec)
		in.EnvVarKeys = extractEnvVarKeys(res.spec)
		in.ConfigMapRefs = extractConfigMapRefs(res.spec)
		in.SecretRefs = extractSecretRefs(res.spec)
	}
	if res.configMapRefs != nil {
		in.ConfigMapRefs = append([]string{}, res.configMapRefs...)
	}
	if res.secretRefs != nil {
		in.SecretRefs = append([]string{}, res.secretRefs...)
	}
	if res.serviceMappings != nil {
		in.ServiceMappings = append([]string{}, res.serviceMappings...)
	}
	if res.ingressRules != nil {
		in.IngressRules = append([]string{}, res.ingressRules...)
	}
}

func lastModifiedFromAnnotations(ann map[string]string) (by string, at time.Time, op string) {
	if len(ann) == constants.DefaultInitValue {
		return constants.EmptyString, time.Time{}, constants.EmptyString
	}
	by = strings.TrimSpace(ann[constants.AnnotationLastModifiedBy])
	op = strings.TrimSpace(ann[constants.AnnotationLastModifiedOperation])
	atRaw := strings.TrimSpace(ann[constants.AnnotationLastModifiedAt])
	at = apputils.ParseRFC3339OrNano(atRaw)
	return by, at, op
}

func buildEnrichCache(ctx context.Context, inputs []derivation.ResourceInput) map[string]enrichResult {
	type nskind struct{ ns, kind string }
	seen := make(map[nskind]bool)
	cache := make(map[string]enrichResult, len(inputs))
	for _, r := range inputs {
		if res, ok := enrichFromCachedManifest(r.Kind, r.Name, r.Namespace); ok {
			cache[r.Namespace+keySeparator+r.Kind+keySeparator+r.Name] = res
			continue
		}
		seen[nskind{r.Namespace, r.Kind}] = true
	}
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	for nk := range seen {
		nk := nk
		g.Go(func() error {
			results := fetchByNamespaceKind(gctx, nk.ns, nk.kind)
			mu.Lock()
			for name, res := range results {
				cache[nk.ns+keySeparator+nk.kind+keySeparator+name] = res
			}
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()
	return cache
}

// Kinds without a fetcher stay unenriched so the informer set (which is
// wider) does not start stamping fields the live path never produced.
func enrichFromCachedManifest(kind, name, ns string) (enrichResult, bool) {
	if GetManifestFromCache == nil {
		return enrichResult{}, false
	}
	if _, known := kindFetchers[kind]; !known {
		return enrichResult{}, false
	}
	obj, ok := GetManifestFromCache(kind, name, ns)
	if !ok {
		return enrichResult{}, false
	}
	u := unstructured.Unstructured{Object: obj}
	res := baseEnrichResult(u.GetAnnotations(), u.GetCreationTimestamp().Time)
	switch kind {
	case appshared.KindDeployment, appshared.KindStatefulSet, appshared.KindDaemonSet:
		res.spec = podSpecFromUnstructured(&u)
	case appshared.KindService:
		var s corev1.Service
		if runtime.DefaultUnstructuredConverter.FromUnstructured(obj, &s) == nil {
			res.serviceMappings = extractServiceMappings(&s)
		}
	case appshared.KindIngress:
		var ing networkingv1.Ingress
		if runtime.DefaultUnstructuredConverter.FromUnstructured(obj, &ing) == nil {
			res.ingressRules = extractIngressRules(&ing)
		}
	default:
	}
	return res, true
}

func podSpecFromUnstructured(u *unstructured.Unstructured) *corev1.PodSpec {
	m, found, err := unstructured.NestedMap(
		u.Object, constants.K8sObjectFieldSpec, constants.K8sObjectFieldTemplate, constants.K8sObjectFieldSpec,
	)
	if err != nil || !found {
		return nil
	}
	var spec corev1.PodSpec
	if runtime.DefaultUnstructuredConverter.FromUnstructured(m, &spec) != nil {
		return nil
	}
	return &spec
}

func baseEnrichResult(annotations map[string]string, created time.Time) enrichResult {
	by, at, op := lastModifiedFromAnnotations(annotations)
	return enrichResult{
		createdAt:      created,
		lastModifiedBy: by,
		lastModifiedAt: at,
		lastModifiedOp: op,
	}
}

func fetchByNamespaceKind(ctx context.Context, ns, kind string) map[string]enrichResult {
	if err := ctx.Err(); err != nil {
		return nil
	}
	// Called directly, not in a ctx-guarded goroutine: a K8s call that ignores ctx
	// would outlive the timer and leak. The REST layer enforces the per-request timeout.
	return fetchByNamespaceKindSync(ns, kind)
}

type kindFetcher func(ns string) map[string]enrichResult

var kindFetchers = map[string]kindFetcher{
	appshared.KindDeployment:              fetchDeploymentsByNs,
	appshared.KindStatefulSet:             fetchStatefulSetsByNs,
	appshared.KindDaemonSet:               fetchDaemonSetsByNs,
	appshared.KindJob:                     fetchJobsByNs,
	appshared.KindCronJob:                 fetchCronJobsByNs,
	appshared.KindService:                 fetchServicesByNs,
	appshared.KindConfigMap:               fetchConfigMapsByNs,
	appshared.KindSecret:                  fetchSecretsByNs,
	appshared.KindServiceAccount:          fetchServiceAccountsByNs,
	appshared.KindPersistentVolumeClaim:   fetchPVCsByNs,
	appshared.KindNetworkPolicy:           fetchNetworkPoliciesByNs,
	appshared.KindIngress:                 fetchIngressesByNs,
	appshared.KindHorizontalPodAutoscaler: fetchHPAsByNs,
}

func fetchByNamespaceKindSync(ns, kind string) map[string]enrichResult {
	if fn, ok := kindFetchers[kind]; ok {
		return fn(ns)
	}
	return nil
}

func fetchDeploymentsByNs(ns string) map[string]enrichResult {
	list, err := workload.GetDeploymentsByNamespace(ns)
	if err != nil {
		return nil
	}
	out := make(map[string]enrichResult, len(list))
	for i := range list {
		d := &list[i]
		spec := d.Spec.Template.Spec
		r := baseEnrichResult(d.Annotations, d.CreationTimestamp.Time)
		r.spec = &spec
		out[d.Name] = r
	}
	return out
}

func fetchStatefulSetsByNs(ns string) map[string]enrichResult {
	list, err := workload.GetStatefulSetsByNamespace(ns)
	if err != nil {
		return nil
	}
	out := make(map[string]enrichResult, len(list))
	for i := range list {
		s := &list[i]
		spec := s.Spec.Template.Spec
		r := baseEnrichResult(s.Annotations, s.CreationTimestamp.Time)
		r.spec = &spec
		out[s.Name] = r
	}
	return out
}

func fetchDaemonSetsByNs(ns string) map[string]enrichResult {
	list, err := workload.GetDaemonSetsByNamespace(ns)
	if err != nil {
		return nil
	}
	out := make(map[string]enrichResult, len(list))
	for i := range list {
		d := &list[i]
		spec := d.Spec.Template.Spec
		r := baseEnrichResult(d.Annotations, d.CreationTimestamp.Time)
		r.spec = &spec
		out[d.Name] = r
	}
	return out
}

func fetchJobsByNs(ns string) map[string]enrichResult {
	list, err := workload.GetJobsByNamespace(ns)
	if err != nil {
		return nil
	}
	out := make(map[string]enrichResult, len(list))
	for i := range list {
		j := &list[i]
		out[j.Name] = baseEnrichResult(j.Annotations, j.CreationTimestamp.Time)
	}
	return out
}

func fetchCronJobsByNs(ns string) map[string]enrichResult {
	list, err := workload.GetCronJobsByNamespace(ns)
	if err != nil {
		return nil
	}
	out := make(map[string]enrichResult, len(list))
	for i := range list {
		c := &list[i]
		out[c.Name] = baseEnrichResult(c.Annotations, c.CreationTimestamp.Time)
	}
	return out
}

func fetchServicesByNs(ns string) map[string]enrichResult {
	list, err := networking.GetServicesByNamespace(ns)
	if err != nil {
		return nil
	}
	out := make(map[string]enrichResult, len(list))
	for i := range list {
		s := &list[i]
		r := baseEnrichResult(s.Annotations, s.CreationTimestamp.Time)
		r.serviceMappings = extractServiceMappings(s)
		out[s.Name] = r
	}
	return out
}

func fetchConfigMapsByNs(ns string) map[string]enrichResult {
	list, err := core.GetConfigMapsByNamespace(ns)
	if err != nil {
		return nil
	}
	out := make(map[string]enrichResult, len(list))
	for i := range list {
		c := &list[i]
		out[c.Name] = baseEnrichResult(c.Annotations, c.CreationTimestamp.Time)
	}
	return out
}

func fetchSecretsByNs(ns string) map[string]enrichResult {
	list, err := core.GetSecretsByNamespace(ns)
	if err != nil {
		return nil
	}
	out := make(map[string]enrichResult, len(list))
	for i := range list {
		s := &list[i]
		out[s.Name] = baseEnrichResult(s.Annotations, s.CreationTimestamp.Time)
	}
	return out
}

func fetchServiceAccountsByNs(ns string) map[string]enrichResult {
	list, err := core.GetServiceAccountsByNamespace(ns)
	if err != nil {
		return nil
	}
	out := make(map[string]enrichResult, len(list))
	for i := range list {
		s := &list[i]
		out[s.Name] = baseEnrichResult(s.Annotations, s.CreationTimestamp.Time)
	}
	return out
}

func fetchPVCsByNs(ns string) map[string]enrichResult {
	list, err := core.GetPersistentVolumeClaimsByNamespace(ns)
	if err != nil {
		return nil
	}
	out := make(map[string]enrichResult, len(list))
	for i := range list {
		p := &list[i]
		out[p.Name] = baseEnrichResult(p.Annotations, p.CreationTimestamp.Time)
	}
	return out
}

func fetchNetworkPoliciesByNs(ns string) map[string]enrichResult {
	list, err := networking.GetNetworkPoliciesByNamespace(ns)
	if err != nil {
		return nil
	}
	out := make(map[string]enrichResult, len(list))
	for i := range list {
		n := &list[i]
		out[n.Name] = baseEnrichResult(n.Annotations, n.CreationTimestamp.Time)
	}
	return out
}

func fetchIngressesByNs(ns string) map[string]enrichResult {
	list, err := networking.GetIngressesByNamespace(ns)
	if err != nil {
		return nil
	}
	out := make(map[string]enrichResult, len(list))
	for i := range list {
		ing := &list[i]
		r := baseEnrichResult(ing.Annotations, ing.CreationTimestamp.Time)
		r.ingressRules = extractIngressRules(ing)
		out[ing.Name] = r
	}
	return out
}

func fetchHPAsByNs(ns string) map[string]enrichResult {
	list, err := autoscaling.GetHorizontalPodAutoscalersByNamespace(ns)
	if err != nil {
		return nil
	}
	out := make(map[string]enrichResult, len(list))
	for i := range list {
		h := &list[i]
		out[h.Name] = baseEnrichResult(h.Annotations, h.CreationTimestamp.Time)
	}
	return out
}

func extractImages(spec *corev1.PodSpec) []string {
	seen := make(map[string]bool)
	for _, c := range spec.InitContainers {
		if c.Image != constants.EmptyString {
			seen[c.Image] = true
		}
	}
	for _, c := range spec.Containers {
		if c.Image != constants.EmptyString {
			seen[c.Image] = true
		}
	}
	out := make([]string, constants.DefaultInitValue, len(seen))
	for img := range seen {
		out = append(out, img)
	}
	return out
}

func extractPorts(spec *corev1.PodSpec) []int {
	seen := make(map[int]bool)
	for _, c := range spec.Containers {
		for _, p := range c.Ports {
			seen[int(p.ContainerPort)] = true
		}
	}
	out := make([]int, constants.DefaultInitValue, len(seen))
	for port := range seen {
		out = append(out, port)
	}
	return out
}

func extractEnvVarKeys(spec *corev1.PodSpec) []string {
	seen := make(map[string]bool)
	for _, c := range spec.Containers {
		for _, e := range c.Env {
			key := e.Name
			if key == constants.EmptyString {
				continue
			}
			if envVarMatchesSkip(strings.ToUpper(key), envVarSkipUpper) {
				continue
			}
			seen[key] = true
		}
	}
	out := make([]string, constants.DefaultInitValue, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	return out
}

func extractConfigMapRefs(spec *corev1.PodSpec) []string {
	seen := make(map[string]bool)
	for _, v := range spec.Volumes {
		if v.ConfigMap != nil && v.ConfigMap.Name != constants.EmptyString {
			seen[v.ConfigMap.Name] = true
		}
	}
	for _, c := range spec.InitContainers {
		extractContainerConfigMapRefs(c, seen)
	}
	for _, c := range spec.Containers {
		extractContainerConfigMapRefs(c, seen)
	}
	out := make([]string, constants.DefaultInitValue, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	return out
}

func extractContainerConfigMapRefs(c corev1.Container, seen map[string]bool) {
	for _, e := range c.Env {
		if e.ValueFrom != nil && e.ValueFrom.ConfigMapKeyRef != nil {
			name := e.ValueFrom.ConfigMapKeyRef.Name
			if name != constants.EmptyString {
				seen[name] = true
			}
		}
	}
	for _, from := range c.EnvFrom {
		if from.ConfigMapRef != nil && from.ConfigMapRef.Name != constants.EmptyString {
			seen[from.ConfigMapRef.Name] = true
		}
	}
}

func extractSecretRefs(spec *corev1.PodSpec) []string {
	seen := make(map[string]bool)
	for _, v := range spec.Volumes {
		if v.Secret != nil && v.Secret.SecretName != constants.EmptyString {
			seen[v.Secret.SecretName] = true
		}
	}
	for _, c := range spec.InitContainers {
		extractContainerSecretRefs(c, seen)
	}
	for _, c := range spec.Containers {
		extractContainerSecretRefs(c, seen)
	}
	out := make([]string, constants.DefaultInitValue, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	return out
}

func extractContainerSecretRefs(c corev1.Container, seen map[string]bool) {
	for _, e := range c.Env {
		if e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
			name := e.ValueFrom.SecretKeyRef.Name
			if name != constants.EmptyString {
				seen[name] = true
			}
		}
	}
	for _, from := range c.EnvFrom {
		if from.SecretRef != nil && from.SecretRef.Name != constants.EmptyString {
			seen[from.SecretRef.Name] = true
		}
	}
}

func extractServiceMappings(s *corev1.Service) []string {
	out := make([]string, constants.DefaultInitValue, len(s.Spec.Ports))
	for _, p := range s.Spec.Ports {
		out = append(
			out,
			string(s.Spec.Type)+constants.ColonSeparator+string(p.Protocol)+constants.ColonSeparator+
				formatServicePort(p.Port, p.TargetPort.String()),
		)
	}
	return out
}

func formatServicePort(port int32, targetPort string) string {
	return strconv.Itoa(int(port)) + constants.ColonSeparator + strings.TrimSpace(targetPort)
}

func extractIngressRules(ing *networkingv1.Ingress) []string {
	out := make([]string, constants.DefaultInitValue, len(ing.Spec.Rules))
	for _, rule := range ing.Spec.Rules {
		out = append(out, ingressRuleValues(rule)...)
	}
	return out
}

func ingressRuleValues(rule networkingv1.IngressRule) []string {
	host := strings.TrimSpace(rule.Host)
	if rule.HTTP == nil {
		return []string{host}
	}
	out := make([]string, constants.DefaultInitValue, len(rule.HTTP.Paths))
	for _, path := range rule.HTTP.Paths {
		backend := strings.TrimSpace(path.Backend.Service.Name)
		out = append(
			out,
			host+constants.ColonSeparator+path.Path+constants.ColonSeparator+backend,
		)
	}
	return out
}

func envVarMatchesSkip(upper string, skipUpper []string) bool {
	return slices.ContainsFunc(skipUpper, func(s string) bool {
		return s != constants.EmptyString && strings.Contains(upper, s)
	})
}
