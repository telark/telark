package api

import (
	"context"
	"fmt"
	"time"

	"github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/messages"
	"github.com/telark/telark/internal/data/metadata/base"
	crdutils "github.com/telark/telark/internal/kcore/crds/utils"
	"github.com/telark/telark/internal/kcore/resilience/timeout"
	"github.com/telark/telark/internal/kcore/shared"
	"k8s.io/client-go/dynamic"
)

type preparedCall struct {
	ctx         context.Context //nolint:containedctx // single-call carrier consumed within callsite.
	cancel      context.CancelFunc
	client      dynamic.ResourceInterface
	errEnvelope shared.KubernetesAPIData
	ok          bool
}

func prepareNamed(name string, metadata base.Metadata, opTimeout time.Duration) preparedCall {
	if vErr := crdutils.ValidateResourceName(name); vErr != nil {
		return preparedCall{
			errEnvelope: shared.CreateKubernetesAPIData(
				shared.StatusBadRequest,
				string(errors.ErrResourceNameCannotBeEmpty),
				nil, vErr,
			),
		}
	}
	return prepare(metadata, opTimeout)
}

func prepare(metadata base.Metadata, opTimeout time.Duration) preparedCall {
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
