package workload

import (
	"context"

	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/k8sclient"
	"github.com/plsyro/kcore/resilience/timeout"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func WorkloadPodMatchLabels(namespace, kind, name string) map[string]string {
	if namespace == constants.EmptyString || name == constants.EmptyString {
		return nil
	}
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil
	}
	ctx, cancel := timeout.ContextWithTimeoutCause(constants.WorkloadGetTimeout)
	defer cancel()

	return returnMatchLabels(ctx, client, namespace, kind, name)
}

func returnMatchLabels(
	ctx context.Context,
	client kubernetes.Interface,
	namespace, kind, name string,
) map[string]string {
	switch kind {
	case KindDeployment:
		return deploymentMatchLabels(ctx, client, namespace, name)
	case KindStatefulSet:
		return statefulSetMatchLabels(ctx, client, namespace, name)
	case KindDaemonSet:
		return daemonSetMatchLabels(ctx, client, namespace, name)
	default:
		return nil
	}
}

func deploymentMatchLabels(ctx context.Context, c kubernetes.Interface, ns, name string) map[string]string {
	d, err := c.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil || d.Spec.Selector == nil {
		return nil
	}
	return d.Spec.Selector.MatchLabels
}

func statefulSetMatchLabels(ctx context.Context, c kubernetes.Interface, ns, name string) map[string]string {
	s, err := c.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil || s.Spec.Selector == nil {
		return nil
	}
	return s.Spec.Selector.MatchLabels
}

func daemonSetMatchLabels(ctx context.Context, c kubernetes.Interface, ns, name string) map[string]string {
	d, err := c.AppsV1().DaemonSets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil || d.Spec.Selector == nil {
		return nil
	}
	return d.Spec.Selector.MatchLabels
}
