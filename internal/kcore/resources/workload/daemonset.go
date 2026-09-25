package workload

import (
	"github.com/telark/kcore/constants"
	"github.com/telark/kcore/k8sclient"
	k8sappsv1 "k8s.io/api/apps/v1"
)

func GetDaemonSetsByNamespace(namespace string) ([]k8sappsv1.DaemonSet, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, err
	}

	daemons, err := getDaemonOrStatefulWithTimeout(daemonItems(client, namespace))
	if err != nil {
		logWorkloadFetchError(constants.ErrFailedToFetchDaemonsets, namespace, err)
		return nil, err
	}
	return daemons, nil
}

func CheckDaemonSetExists(namespace, name string) (bool, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return false, err
	}

	return isWorkloadPresent(daemonGet(client, namespace, name))
}
