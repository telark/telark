package core

import (
	"fmt"

	"github.com/telark/kcore/constants"
	"github.com/telark/kcore/k8sclient"
	"github.com/telark/kcore/resilience/timeout"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetStorageClasses() ([]storagev1.StorageClass, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := timeout.ContextWithTimeoutCause(constants.DefaultTimeout)
	defer cancel()

	scs, err := client.StorageV1().StorageClasses().List(ctx, k8smetav1.ListOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToFetchStorage), err))
		return nil, err
	}
	return scs.Items, nil
}

func GetPersistentVolumeClaims(namespace string) ([]corev1.PersistentVolumeClaim, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := timeout.ContextWithTimeoutCause(constants.DefaultTimeout)
	defer cancel()

	pvcs, err := client.CoreV1().PersistentVolumeClaims(namespace).List(ctx, k8smetav1.ListOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToFetchStorage), err))
		return nil, err
	}
	return pvcs.Items, nil
}

func GetPersistentVolumes() ([]corev1.PersistentVolume, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := timeout.ContextWithTimeoutCause(constants.DefaultTimeout)
	defer cancel()

	pvs, err := client.CoreV1().PersistentVolumes().List(ctx, k8smetav1.ListOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToFetchStorage), err))
		return nil, err
	}
	return pvs.Items, nil
}
