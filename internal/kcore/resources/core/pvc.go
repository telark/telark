package core

import (
	"fmt"

	"github.com/telark/kcore/constants"
	"github.com/telark/kcore/k8sclient"
	"github.com/telark/kcore/resilience/timeout"
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
		return constants.ZeroValue, err
	}

	ctx, cancel := timeout.ContextWithTimeoutCause(constants.DefaultTimeout)
	defer cancel()

	pvc, err := client.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToGetPVC), name, namespace, err))
		return constants.ZeroValue, err
	}

	storage, exists := pvc.Status.Capacity[k8scorev1.ResourceStorage]
	if !exists {
		return constants.ZeroValue, fmt.Errorf(string(constants.ErrPVCStorageCapacityMissing), name, namespace)
	}

	totalBytes, ok := storage.AsInt64()
	if !ok || totalBytes < constants.ZeroValue {
		return constants.ZeroValue, fmt.Errorf(string(constants.ErrPVCStorageCapacityMissing), name, namespace)
	}

	return totalBytes, nil
}
