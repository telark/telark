package api

import (
	"context"

	"github.com/telark/data/metadata/base"
	"github.com/telark/kcore/shared"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

func UpdateCustomResource(
	name string,
	metadata base.Metadata,
	template *unstructured.Unstructured,
) shared.KubernetesAPIData {
	return performRetryingWrite(name, metadata,
		func(ctx context.Context, client dynamic.ResourceInterface) (*unstructured.Unstructured, error) {
			// Refresh ResourceVersion each attempt: K8s conflicts on stale RV; the
			// re-fetch lets RetryOnConflict succeed against the latest revision.
			if current, getErr := client.Get(ctx, name, k8smetav1.GetOptions{}); getErr == nil {
				template.SetResourceVersion(current.GetResourceVersion())
			}
			return client.Update(ctx, template, k8smetav1.UpdateOptions{})
		},
	)
}
