package core

import (
	"fmt"

	"github.com/telark/telark/internal/kcore/constants"
	"github.com/telark/telark/internal/kcore/k8sclient"
	"github.com/telark/telark/internal/kcore/resilience/timeout"
	k8scorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

var pvcGVR = schema.GroupVersionResource{Version: constants.APIVersionV1, Resource: constants.PVCResource}

// Through the dynamic client, the one tests can fake (k8sclient.SetDynamicClient).
func GetPersistentVolumeClaimCapacityBytes(namespace string, name string) (int64, error) {
	client, err := k8sclient.InitDynamicClient()
	if err != nil {
		return constants.ZeroValue, err
	}

	ctx, cancel := timeout.ContextWithTimeoutCause(constants.DefaultTimeout)
	defer cancel()

	pvc, err := client.Resource(pvcGVR).Namespace(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToGetPVC), name, namespace, err))
		return constants.ZeroValue, err
	}

	storage, _, _ := unstructured.NestedString(pvc.Object, constants.CrdStatusField, constants.PVCCapacityField, string(k8scorev1.ResourceStorage))
	quantity, err := resource.ParseQuantity(storage)
	totalBytes, ok := quantity.AsInt64()
	if err != nil || !ok || totalBytes < constants.ZeroValue {
		return constants.ZeroValue, fmt.Errorf(string(constants.ErrPVCStorageCapacityMissing), name, namespace)
	}

	return totalBytes, nil
}
