package api

import (
	"github.com/plsyro/data/errors"
	"github.com/plsyro/data/messages"
	"github.com/plsyro/data/metadata/base"
	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/shared"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetCustomResourceByName(name string, metadata base.Metadata) shared.KubernetesAPIData {
	prep := prepare(name, metadata, constants.CrdGetTimeout)
	if !prep.ok {
		return prep.errEnvelope
	}
	defer prep.cancel()

	resource, err := prep.client.Get(prep.ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		return errorEnvelope(errors.ErrGetRes, name, err)
	}
	return okEnvelope(messages.SuccessGetRes, name, metadata.Kind, resource)
}

func ListCustomResources(metadata base.Metadata) shared.KubernetesAPIData {
	prep := prepare("", metadata, constants.CrdListTimeout)
	if !prep.ok {
		return prep.errEnvelope
	}
	defer prep.cancel()

	resourceList, err := prep.client.List(prep.ctx, k8smetav1.ListOptions{})
	if err != nil {
		return errorEnvelopeNoName(errors.ErrListRes, err)
	}
	return shared.CreateKubernetesAPIData(shared.StatusOK, string(messages.SuccessListRes), resourceList, nil)
}
