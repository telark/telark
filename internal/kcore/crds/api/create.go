package api

import (
	"github.com/plsyro/data/errors"
	"github.com/plsyro/data/messages"
	"github.com/plsyro/data/metadata/base"
	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/shared"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func CreateCustomResource(template *unstructured.Unstructured, metadata base.Metadata) shared.KubernetesAPIData {
	prep := prepare("", metadata, constants.CrdCreateTimeout)
	if !prep.ok {
		return prep.errEnvelope
	}
	defer prep.cancel()

	resource, err := prep.client.Create(prep.ctx, template, k8smetav1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			return shared.CreateKubernetesAPIData(shared.StatusConflict, string(errors.ErrResExists), nil, nil)
		}
		return errorEnvelope(errors.ErrCreateRes, template.GetName(), err)
	}
	return okEnvelope(messages.SuccessCreateRes, template.GetName(), metadata.Kind, resource)
}
