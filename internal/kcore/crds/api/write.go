package api

import (
	"context"

	"github.com/telark/data/errors"
	"github.com/telark/data/messages"
	"github.com/telark/data/metadata/base"
	"github.com/telark/kcore/constants"
	"github.com/telark/kcore/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
)

// writeFn performs one attempt of a Patch/Update against the supplied resource
// client and ctx. Returns the post-write object or the K8s error.
type writeFn func(ctx context.Context, client dynamic.ResourceInterface) (*unstructured.Unstructured, error)

// performRetryingWrite is the shared pipeline for named CRD writes
// (Patch, Update): prepare ctx+client, retry op on K8s conflict under
// retry.DefaultRetry, wrap result in the project's envelope.
func performRetryingWrite(name string, metadata base.Metadata, op writeFn) shared.KubernetesAPIData {
	prep := prepare(name, metadata, constants.CrdPatchTimeout)
	if !prep.ok {
		return prep.errEnvelope
	}
	defer prep.cancel()

	var resource *unstructured.Unstructured
	if rErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var opErr error
		resource, opErr = op(prep.ctx, prep.client)
		return opErr
	}); rErr != nil {
		return errorEnvelope(errors.ErrUpdateRes, name, rErr)
	}
	return okEnvelope(messages.SuccessUpdateRes, name, metadata.Kind, resource)
}
