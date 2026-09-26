package api

import (
	"context"
	"encoding/json"

	"github.com/telark/data/errors"
	"github.com/telark/data/messages"
	"github.com/telark/data/metadata/base"
	"github.com/telark/kcore/constants"
	kretry "github.com/telark/kcore/resilience/retry"
	"github.com/telark/kcore/shared"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
)

func PatchCustomResourceStatus(metadata base.Metadata, name string, payload map[string]any) shared.KubernetesAPIData {
	patchBytes, err := json.Marshal(payload)
	if err != nil {
		return shared.CreateKubernetesAPIData(
			shared.StatusInternalServerError,
			string(errors.ErrRestMarshalPayload),
			nil, err)
	}
	return performRetryingWrite(name, metadata, statusPatchFn(name, patchBytes))
}

func UpdateCustomResourceStatus(
	name string,
	metadata base.Metadata,
	template *unstructured.Unstructured,
) shared.KubernetesAPIData {
	return performRetryingWrite(name, metadata,
		func(ctx context.Context, client dynamic.ResourceInterface) (*unstructured.Unstructured, error) {
			if current, getErr := client.Get(ctx, name, k8smetav1.GetOptions{}); getErr == nil {
				template.SetResourceVersion(current.GetResourceVersion())
			}
			return client.UpdateStatus(ctx, template, k8smetav1.UpdateOptions{})
		},
	)
}

// The API server drops .status on create when the CRD has a status subresource,
// so a kind with projected status fields needs a second write.
func CreateCustomResourceWithStatus(template *unstructured.Unstructured, metadata base.Metadata) shared.KubernetesAPIData {
	status, hasStatus := template.Object[constants.CrdStatusField].(map[string]any)
	if len(metadata.StatusFields) == constants.ZeroValue || !hasStatus || len(status) == constants.ZeroValue {
		return CreateCustomResource(template, metadata)
	}

	created := CreateCustomResource(template, metadata)
	if created.Status != shared.StatusOK {
		return created
	}

	name := template.GetName()
	patchBytes, err := json.Marshal(map[string]any{constants.CrdStatusField: status})
	if err != nil {
		return errorEnvelope(errors.ErrPatchRes, name, err)
	}
	patched := performWrite(name, metadata, statusCreateBackoff(), isStatusAfterCreateRetriable, statusPatchFn(name, patchBytes))
	if patched.Status != shared.StatusOK {
		return errorEnvelope(errors.ErrPatchRes, name, patched.Error)
	}
	return okEnvelope(messages.SuccessCreateRes, name, metadata.Kind, patched.Data)
}

func statusPatchFn(name string, patchBytes []byte) writeFn {
	return func(ctx context.Context, client dynamic.ResourceInterface) (*unstructured.Unstructured, error) {
		return client.Patch(ctx, name, types.MergePatchType, patchBytes, k8smetav1.PatchOptions{}, constants.CrdStatusSubresource)
	}
}

func statusCreateBackoff() wait.Backoff {
	b := kretry.DefaultTransient()
	return wait.Backoff{Steps: b.Steps, Duration: b.Duration, Factor: b.Factor, Jitter: b.Jitter, Cap: b.Cap}
}

// A read-after-create can briefly miss the object behind a lagging cache.
func isStatusAfterCreateRetriable(err error) bool {
	return k8serrors.IsConflict(err) || k8serrors.IsNotFound(err) || kretry.IsTransientK8sError(err)
}
