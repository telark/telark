package workload

import (
	"context"
	"fmt"

	dataerrors "github.com/telark/data/errors"
	"github.com/telark/kcore/constants"
	"github.com/telark/kcore/k8sclient"
	"github.com/telark/kcore/resilience/timeout"
	k8sappsv1 "k8s.io/api/apps/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclientgo "k8s.io/client-go/kubernetes"
)

func getDaemonOrStatefulWithTimeout[T any](
	listFn func(context.Context) ([]T, error),
) ([]T, error) {
	ctx, cancel := timeout.ContextWithTimeoutCause(constants.WorkloadListTimeout)
	defer cancel()
	return listFn(ctx)
}

func logWorkloadFetchError(message dataerrors.Error, namespace string, err error) {
	k8sclient.GetLogger().Error(fmt.Sprintf(string(message), namespace, err))
}

func isWorkloadPresent(getFn func(context.Context) error) (bool, error) {
	ctx, cancel := timeout.ContextWithTimeoutCause(constants.WorkloadGetTimeout)
	defer cancel()
	if err := getFn(ctx); err != nil {
		return false, nil
	}
	return true, nil
}

func daemonGet(
	client k8sclientgo.Interface,
	namespace string,
	name string,
) func(context.Context) error {
	return func(ctx context.Context) error {
		_, err := client.AppsV1().DaemonSets(namespace).Get(
			ctx,
			name,
			k8smetav1.GetOptions{},
		)
		return err
	}
}

func statefulGet(
	client k8sclientgo.Interface,
	namespace string,
	name string,
) func(context.Context) error {
	return func(ctx context.Context) error {
		_, err := client.AppsV1().StatefulSets(namespace).Get(
			ctx,
			name,
			k8smetav1.GetOptions{},
		)
		return err
	}
}

func daemonItems(
	client k8sclientgo.Interface,
	namespace string,
) func(context.Context) ([]k8sappsv1.DaemonSet, error) {
	return func(ctx context.Context) ([]k8sappsv1.DaemonSet, error) {
		list, err := client.AppsV1().DaemonSets(namespace).List(
			ctx,
			k8smetav1.ListOptions{},
		)
		if err != nil {
			return nil, err
		}
		return list.Items, nil
	}
}

func statefulItems(
	client k8sclientgo.Interface,
	namespace string,
) func(context.Context) ([]k8sappsv1.StatefulSet, error) {
	return func(ctx context.Context) ([]k8sappsv1.StatefulSet, error) {
		list, err := client.AppsV1().StatefulSets(namespace).List(
			ctx,
			k8smetav1.ListOptions{},
		)
		if err != nil {
			return nil, err
		}
		return list.Items, nil
	}
}
