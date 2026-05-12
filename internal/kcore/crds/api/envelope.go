package api

import (
	"context"
	"fmt"
	"time"

	"github.com/plsyro/data/errors"
	"github.com/plsyro/data/messages"
	"github.com/plsyro/data/metadata/base"
	crdutils "github.com/plsyro/kcore/crds/utils"
	"github.com/plsyro/kcore/resilience/timeout"
	"github.com/plsyro/kcore/shared"
	"k8s.io/client-go/dynamic"
)

// preparedCall bundles the per-call wiring shared by every CRD operation:
// timed context, dynamic resource client. When ok is false, errEnvelope
// holds the populated KubernetesAPIData the caller must return as-is.
type preparedCall struct {
	ctx         context.Context //nolint:containedctx // single-call carrier consumed within callsite.
	cancel      context.CancelFunc
	client      dynamic.ResourceInterface
	errEnvelope shared.KubernetesAPIData
	ok          bool
}

// prepare validates name (when nonempty), resolves the dynamic client, and
// builds the timed context. On failure, returns ok=false with errEnvelope
// populated; on success, caller MUST `defer prep.cancel()`.
func prepare(name string, metadata base.Metadata, opTimeout time.Duration) preparedCall {
	if name != "" {
		if vErr := crdutils.ValidateResourceName(name); vErr != nil {
			return preparedCall{
				errEnvelope: shared.CreateKubernetesAPIData(
					shared.StatusBadRequest,
					string(errors.ErrResourceNameCannotBeEmpty),
					nil, vErr,
				),
			}
		}
	}
	client, cErr := crdutils.GetResourceClient(metadata)
	if cErr != nil {
		return preparedCall{errEnvelope: shared.HandleClientError(cErr)}
	}
	ctx, cancel := timeout.ContextWithTimeoutCause(opTimeout)
	return preparedCall{ctx: ctx, cancel: cancel, client: client, ok: true}
}

func errorEnvelope(tpl errors.Error, name string, err error) shared.KubernetesAPIData {
	return shared.CreateKubernetesAPIData(
		shared.StatusInternalServerError,
		fmt.Sprintf(string(tpl), name, err),
		nil, err,
	)
}

func errorEnvelopeNoName(tpl errors.Error, err error) shared.KubernetesAPIData {
	return shared.CreateKubernetesAPIData(
		shared.StatusInternalServerError,
		fmt.Sprintf(string(tpl), err),
		nil, err,
	)
}

func okEnvelope(tpl messages.Message, name, kind string, resource any) shared.KubernetesAPIData {
	return shared.CreateKubernetesAPIData(
		shared.StatusOK,
		fmt.Sprintf(string(tpl), name, kind),
		resource, nil,
	)
}
