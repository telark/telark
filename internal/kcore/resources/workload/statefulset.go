package workload

import (
	"github.com/plsyro/kcore/constants"
	k8sappsv1 "k8s.io/api/apps/v1"
)

func GetStatefulSetsByNamespace(namespace string) ([]k8sappsv1.StatefulSet, error) {
	client, err := getAppsClient()
	if err != nil {
		return nil, err
	}

	sets, err := getDaemonOrStatefulWithTimeout(statefulItems(client, namespace))
	if err != nil {
		logWorkloadFetchError(constants.ErrFailedToFetchStatefulsets, namespace, err)
		return nil, err
	}
	return sets, nil
}

func CheckStatefulSetExists(namespace, name string) (bool, error) {
	client, err := getAppsClient()
	if err != nil {
		return false, err
	}

	return isWorkloadPresent(statefulGet(client, namespace, name))
}
