package core

import (
	"errors"
	"fmt"

	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/k8sclient"
	"github.com/plsyro/kcore/resilience/timeout"
	k8scorev1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetPersistentVolumeClaimsByNamespace(namespace string) ([]k8scorev1.PersistentVolumeClaim, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeout.ContextWithTimeoutCause(constants.ResourceListTimeout)
	defer cancel()

	list, err := client.CoreV1().PersistentVolumeClaims(namespace).List(ctx, k8smetav1.ListOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToFetchPVCs), namespace, err))
		return nil, err
	}
	return list.Items, nil
}

func GetPersistentVolumeClaimCapacityBytes(namespace string, name string) (int64, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return 0, err
	}

	ctx, cancel := timeout.ContextWithTimeoutCause(constants.DefaultTimeout)
	defer cancel()

	pvc, err := client.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToGetPVC), name, namespace, err))
		return 0, err
	}

	storage, exists := pvc.Status.Capacity[k8scorev1.ResourceStorage]
	if !exists {
		return 0, errors.New(fmt.Sprintf(string(constants.ErrPVCStorageCapacityMissing), name, namespace))
	}

	totalBytes, ok := storage.AsInt64()
	if !ok || totalBytes < 0 {
		return 0, errors.New(fmt.Sprintf(string(constants.ErrPVCStorageCapacityMissing), name, namespace))
	}

	return totalBytes, nil
}
