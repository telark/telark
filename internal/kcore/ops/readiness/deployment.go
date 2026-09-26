package readiness

import (
	"context"
	"errors"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const defaultDesiredReplicas int32 = 1

func WaitDeploymentReady(
	ctx context.Context,
	kubeClient *kubernetes.Clientset,
	namespace string,
	name string,
	timeout time.Duration,
	pollInterval time.Duration,
) error {
	if kubeClient == nil {
		return errors.New("kube client is nil")
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
			d, err := kubeClient.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				continue
			}
			desired := defaultDesiredReplicas
			if d.Spec.Replicas != nil {
				desired = *d.Spec.Replicas
			}
			if d.Status.ReadyReplicas >= desired {
				return nil
			}
		}
	}
	return errors.New("deployment not ready")
}
