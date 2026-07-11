package api

import (
	"context"
	"encoding/json"

	"github.com/telark/data/errors"
	"github.com/telark/data/metadata/base"
	"github.com/telark/kcore/shared"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

func PatchCustomResource(metadata base.Metadata, name string, payload map[string]any) shared.KubernetesAPIData {
	patchBytes, err := json.Marshal(payload)
	if err != nil {
		return shared.CreateKubernetesAPIData(
			shared.StatusInternalServerError,
			string(errors.ErrRestMarshalPayload),
			nil, err)
	}

	return performRetryingWrite(name, metadata,
		func(ctx context.Context, client dynamic.ResourceInterface) (*unstructured.Unstructured, error) {
			return client.Patch(ctx, name, types.MergePatchType, patchBytes, k8smetav1.PatchOptions{})
		},
	)
}
