package api

import (
	"github.com/telark/data/errors"
	"github.com/telark/data/messages"
	"github.com/telark/data/metadata/base"
	"github.com/telark/kcore/constants"
	"github.com/telark/kcore/shared"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetCustomResourceByName(name string, metadata base.Metadata) shared.KubernetesAPIData {
	prep := prepareNamed(name, metadata, constants.CrdGetTimeout)
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
	prep := prepare(metadata, constants.CrdListTimeout)
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
