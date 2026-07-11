package workload

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/telark/data/resources/application"
	"github.com/telark/kcore/constants"
	"github.com/telark/kcore/k8sclient"
	"github.com/telark/kcore/resilience/timeout"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	KindDeployment  = "Deployment"
	KindStatefulSet = "StatefulSet"
	KindDaemonSet   = "DaemonSet"
)

func ComputeBaselineFingerprint(
	image string,
	replicas int32,
	requestsCPU, requestsMem, limitsCPU, limitsMem string,
) string {
	raw := strings.Join([]string{
		image,
		strconv.FormatInt(int64(replicas), 10),
		requestsCPU,
		requestsMem,
		limitsCPU,
		limitsMem,
	}, "|")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:4])
}

func ReadMetricsBaseline(namespace, kind, name string) application.MetricsBaseline {
	if namespace == constants.EmptyString || name == constants.EmptyString {
		return zeroMetricsBaseline()
	}
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return zeroMetricsBaseline()
	}
	ctx, cancel := timeout.ContextWithTimeoutCause(constants.WorkloadGetTimeout)
	defer cancel()

	switch kind {
	case KindDeployment:
		return readBaselineFromDeployment(ctx, client, namespace, name)
	case KindStatefulSet:
		return readBaselineFromStatefulSet(ctx, client, namespace, name)
	case KindDaemonSet:
		return readBaselineFromDaemonSet(ctx, client, namespace, name)
	default:
		return zeroMetricsBaseline()
	}
}

func zeroMetricsBaseline() application.MetricsBaseline {
	empty := constants.EmptyString
	return buildMetricsBaseline(empty, int32(constants.EmptySliceLength), empty, empty, empty, empty)
}

func buildMetricsBaseline(
	image string,
	replicas int32,
	reqCPU, reqMem, limCPU, limMem string,
) application.MetricsBaseline {
	return application.MetricsBaseline{
		Fingerprint: ComputeBaselineFingerprint(image, replicas, reqCPU, reqMem, limCPU, limMem),
		Replicas:    replicas,
		Requests:    application.ResourceValues{CPU: reqCPU, Memory: reqMem},
		Limits:      application.ResourceValues{CPU: limCPU, Memory: limMem},
	}
}

func readBaselineFromDeployment(
	ctx context.Context,
	client kubernetes.Interface,
	ns, name string,
) application.MetricsBaseline {
	d, err := client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return zeroMetricsBaseline()
	}
	rep := int32(constants.SingleItem)
	if d.Spec.Replicas != nil {
		rep = *d.Spec.Replicas
	}
	res := firstContainerResources(&d.Spec.Template.Spec)
	return buildMetricsBaseline(res.image, rep, res.reqCPU, res.reqMem, res.limCPU, res.limMem)
}

func readBaselineFromStatefulSet(
	ctx context.Context,
	client kubernetes.Interface,
	ns, name string,
) application.MetricsBaseline {
	s, err := client.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return zeroMetricsBaseline()
	}
	rep := int32(constants.SingleItem)
	if s.Spec.Replicas != nil {
		rep = *s.Spec.Replicas
	}
	res := firstContainerResources(&s.Spec.Template.Spec)
	return buildMetricsBaseline(res.image, rep, res.reqCPU, res.reqMem, res.limCPU, res.limMem)
}

func readBaselineFromDaemonSet(
	ctx context.Context,
	client kubernetes.Interface,
	ns, name string,
) application.MetricsBaseline {
	d, err := client.AppsV1().DaemonSets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return zeroMetricsBaseline()
	}
	rep := d.Status.DesiredNumberScheduled
	res := firstContainerResources(&d.Spec.Template.Spec)
	return buildMetricsBaseline(res.image, rep, res.reqCPU, res.reqMem, res.limCPU, res.limMem)
}

type containerResourceStrings struct {
	image, reqCPU, reqMem, limCPU, limMem string
}

func firstContainerResources(spec *corev1.PodSpec) containerResourceStrings {
	empty := constants.EmptyString
	if spec == nil || len(spec.Containers) == constants.EmptySliceLength {
		return containerResourceStrings{empty, empty, empty, empty, empty}
	}
	c := spec.Containers[constants.EmptySliceLength]
	return containerResourceStrings{
		image:  c.Image,
		reqCPU: resQuantityString(c.Resources.Requests, corev1.ResourceCPU),
		reqMem: resQuantityString(c.Resources.Requests, corev1.ResourceMemory),
		limCPU: resQuantityString(c.Resources.Limits, corev1.ResourceCPU),
		limMem: resQuantityString(c.Resources.Limits, corev1.ResourceMemory),
	}
}

func resQuantityString(list corev1.ResourceList, name corev1.ResourceName) string {
	if list == nil {
		return constants.EmptyString
	}
	q, ok := list[name]
	if !ok {
		return constants.EmptyString
	}
	return q.String()
}
