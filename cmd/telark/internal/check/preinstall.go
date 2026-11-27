package check

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/telark/telark/cmd/telark/internal/constants"
	"github.com/telark/telark/cmd/telark/internal/version"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilversion "k8s.io/apimachinery/pkg/util/version"
)

var (
	minKubernetes = utilversion.MustParseGeneric(constants.MinKubernetesVersion)
	modes         = []string{constants.ModeMinimal, constants.ModeStandard, constants.ModePerformance}
	modeSizes     = map[string]modeSize{
		constants.ModeMinimal:     {cpuMillis: constants.MinimalCPUMillis, memoryMiB: constants.MinimalMemoryMiB},
		constants.ModeStandard:    {cpuMillis: constants.StandardCPUMillis, memoryMiB: constants.StandardMemoryMiB},
		constants.ModePerformance: {cpuMillis: constants.PerformanceCPUMillis, memoryMiB: constants.PerformanceMemoryMiB},
	}
)

func (r *runner) preinstall(ctx context.Context) []Result {
	return []Result{
		r.kubernetesVersion(),
		r.permissions(ctx),
		r.storageClass(ctx),
		r.otherKyverno(ctx),
		r.earlierInstall(ctx),
		r.capacity(ctx),
	}
}

func (r *runner) kubernetesVersion() Result {
	info, err := r.core.Discovery().ServerVersion()
	if err != nil {
		return ReadFailed(constants.CheckKubernetesVersion, constants.ReadServerVersion, err)
	}
	current, err := utilversion.ParseGeneric(info.GitVersion)
	if err != nil {
		return ReadFailed(constants.CheckKubernetesVersion, constants.ReadServerVersion, err)
	}
	if current.LessThan(minKubernetes) {
		return Fail(constants.CheckKubernetesVersion, fmt.Sprintf(constants.MsgKubeVersionOld, info.GitVersion))
	}
	return Pass(constants.CheckKubernetesVersion, fmt.Sprintf(constants.MsgKubeVersionOK, info.GitVersion))
}

func (r *runner) permissions(ctx context.Context) Result {
	wanted := slices.Concat(attributes(constants.ChartClusterResources, constants.EmptyString),
		attributes(constants.ChartNamespacedResources, r.namespace))
	if _, err := r.core.CoreV1().Namespaces().Get(ctx, r.namespace, metav1.GetOptions{}); apierrors.IsNotFound(err) {
		wanted = append(wanted, authorizationv1.ResourceAttributes{Verb: constants.VerbCreate, Resource: constants.ResourceNamespaces})
	}
	var denied []string
	for i := range wanted {
		review := &authorizationv1.SelfSubjectAccessReview{Spec: authorizationv1.SelfSubjectAccessReviewSpec{ResourceAttributes: &wanted[i]}}
		answer, err := r.core.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, review, metav1.CreateOptions{})
		if err != nil {
			return ReadFailed(constants.CheckPermissions, constants.ReadPermissions, err)
		}
		if !answer.Status.Allowed {
			denied = append(denied, qualified(wanted[i].Resource, wanted[i].Group))
		}
	}
	if len(denied) > constants.DefaultInitValue {
		return Fail(constants.CheckPermissions, fmt.Sprintf(constants.MsgPermissionsDenied, strings.Join(denied, constants.ListSeparator)))
	}
	return Pass(constants.CheckPermissions, fmt.Sprintf(constants.MsgPermissionsOK, len(wanted)))
}

func attributes(resources, namespace string) []authorizationv1.ResourceAttributes {
	fields := strings.Fields(resources)
	list := make([]authorizationv1.ResourceAttributes, constants.DefaultInitValue, len(fields))
	for _, field := range fields {
		resource, group, _ := strings.Cut(field, constants.GroupSeparator)
		list = append(list, authorizationv1.ResourceAttributes{Verb: constants.VerbCreate, Group: group, Resource: resource, Namespace: namespace})
	}
	return list
}

func qualified(resource, group string) string {
	if group == constants.EmptyString {
		return resource
	}
	return resource + constants.GroupSeparator + group
}

func Modes() []string {
	return slices.Clone(modes)
}

func (r *runner) storageClass(ctx context.Context) Result {
	if r.opts.StorageClass == constants.NoStorageClass {
		return Pass(constants.CheckStorageClass, constants.MsgStorageClassNoClass)
	}
	list, err := r.core.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return ReadFailed(constants.CheckStorageClass, constants.ReadStorageClasses, err)
	}
	if r.opts.StorageClass != constants.EmptyString {
		if slices.ContainsFunc(list.Items, func(class storagev1.StorageClass) bool { return class.Name == r.opts.StorageClass }) {
			return Pass(constants.CheckStorageClass, fmt.Sprintf(constants.MsgStorageClassFound, r.opts.StorageClass))
		}
		return Fail(constants.CheckStorageClass, fmt.Sprintf(constants.MsgStorageClassMissing, r.opts.StorageClass))
	}
	for i := range list.Items {
		annotations := list.Items[i].Annotations
		if annotations[constants.DefaultClassAnnotation] == constants.AnnotationTrue ||
			annotations[constants.BetaDefaultClassAnnot] == constants.AnnotationTrue {
			return Pass(constants.CheckStorageClass, fmt.Sprintf(constants.MsgStorageClassOK, list.Items[i].Name))
		}
	}
	return Fail(constants.CheckStorageClass, constants.MsgStorageClassNone)
}

// Any Kyverno registers webhooks under this label: the bundled one would fight it for them, and Telark needs one without it.
func (r *runner) otherKyverno(ctx context.Context) Result {
	list, err := r.core.AdmissionregistrationV1().ValidatingWebhookConfigurations().List(ctx,
		metav1.ListOptions{LabelSelector: constants.KyvernoWebhookSelector})
	if err != nil {
		return ReadFailed(constants.CheckKyverno, constants.ReadWebhooks, err)
	}
	running := len(list.Items) > constants.DefaultInitValue
	switch {
	case r.opts.ExternalKyverno && running:
		return Pass(constants.CheckKyverno, constants.MsgKyvernoToUse)
	case r.opts.ExternalKyverno:
		return Fail(constants.CheckKyverno, constants.MsgKyvernoNeeded)
	case running:
		return Fail(constants.CheckKyverno, fmt.Sprintf(constants.MsgKyvernoPresent, list.Items[constants.DefaultInitValue].Name))
	}
	return Pass(constants.CheckKyverno, constants.MsgKyvernoAbsent)
}

func (r *runner) earlierInstall(ctx context.Context) Result {
	// Telark's CRDs and admission policies are cluster-wide, so a running install in any namespace blocks another.
	installs, err := r.core.AppsV1().Deployments(metav1.NamespaceAll).List(ctx, metav1.ListOptions{LabelSelector: constants.TelarkSelector})
	if err != nil {
		return ReadFailed(constants.CheckEarlierInstall, constants.ReadDeployments, err)
	}
	if len(installs.Items) > constants.DefaultInitValue {
		running := installs.Items[constants.DefaultInitValue].Namespace
		server := version.Summarize(running, installs.Items)
		return Fail(constants.CheckEarlierInstall, fmt.Sprintf(constants.MsgInstallRunning, server.Version, running))
	}
	crds, err := r.telarkCRDs(ctx)
	if err != nil {
		return ReadFailed(constants.CheckEarlierInstall, constants.ReadCRDs, err)
	}
	claims, err := r.core.CoreV1().PersistentVolumeClaims(r.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return ReadFailed(constants.CheckEarlierInstall, constants.ReadVolumeClaims, err)
	}
	var left []string
	if len(crds) > constants.DefaultInitValue {
		left = append(left, fmt.Sprintf(constants.MsgLeftoverCRDs, len(crds)))
	}
	if len(claims.Items) > constants.DefaultInitValue {
		left = append(left, fmt.Sprintf(constants.MsgLeftoverClaims, len(claims.Items), r.namespace))
	}
	if len(left) == constants.DefaultInitValue {
		return Pass(constants.CheckEarlierInstall, constants.MsgInstallClean)
	}
	return Warn(constants.CheckEarlierInstall, fmt.Sprintf(constants.MsgInstallLeftovers, strings.Join(left, constants.MsgLeftoverJoin)))
}

// Free means allocatable minus what running pods request, on the nodes a pod without tolerations can use.
func (r *runner) capacity(ctx context.Context) Result {
	nodes, err := r.core.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return ReadFailed(constants.CheckNodeCapacity, constants.ReadNodes, err)
	}
	pods, err := r.core.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return ReadFailed(constants.CheckNodeCapacity, constants.ReadPods, err)
	}
	free := freeCapacity(nodes.Items, pods.Items)
	need := modeSizes[r.opts.Mode]
	args := []any{float64(free.cpuMillis) / constants.MillisPerCPU, float64(free.memoryBytes) / constants.BytesPerMiB / constants.MiBPerGiB,
		r.opts.Mode, float64(need.cpuMillis) / constants.MillisPerCPU, float64(need.memoryMiB) / constants.MiBPerGiB}
	if free.cpuMillis >= need.cpuMillis && free.memoryBytes >= need.memoryMiB*constants.BytesPerMiB {
		return Pass(constants.CheckNodeCapacity, fmt.Sprintf(constants.MsgCapacityOK, args...))
	}
	return Warn(constants.CheckNodeCapacity, fmt.Sprintf(constants.MsgCapacityShort, args...))
}

func freeCapacity(nodes []corev1.Node, pods []corev1.Pod) capacity {
	var free capacity
	usable := make(map[string]bool, len(nodes))
	for i := range nodes {
		if !schedulable(&nodes[i]) {
			continue
		}
		usable[nodes[i].Name] = true
		free.cpuMillis += nodes[i].Status.Allocatable.Cpu().MilliValue()
		free.memoryBytes += nodes[i].Status.Allocatable.Memory().Value()
	}
	for i := range pods {
		pod := &pods[i]
		if !usable[pod.Spec.NodeName] || pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		for j := range pod.Spec.Containers {
			requests := pod.Spec.Containers[j].Resources.Requests
			free.cpuMillis -= requests.Cpu().MilliValue()
			free.memoryBytes -= requests.Memory().Value()
		}
	}
	return free
}

func schedulable(node *corev1.Node) bool {
	if node.Spec.Unschedulable {
		return false
	}
	for _, taint := range node.Spec.Taints {
		if taint.Effect == corev1.TaintEffectNoSchedule || taint.Effect == corev1.TaintEffectNoExecute {
			return false
		}
	}
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}
