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

type writeFn func(ctx context.Context, client dynamic.ResourceInterface) (*unstructured.Unstructured, error)

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
