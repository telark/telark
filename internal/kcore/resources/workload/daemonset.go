package workload

import (
	"github.com/plsyro/kcore/constants"
	k8sappsv1 "k8s.io/api/apps/v1"
)

func GetDaemonSetsByNamespace(namespace string) ([]k8sappsv1.DaemonSet, error) {
	client, err := getAppsClient()
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
	client, err := getAppsClient()
	if err != nil {
		return false, err
	}

	return isWorkloadPresent(daemonGet(client, namespace, name))
}
