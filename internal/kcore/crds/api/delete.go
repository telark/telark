package api

import (
	"github.com/plsyro/data/errors"
	"github.com/plsyro/data/messages"
	"github.com/plsyro/data/metadata/base"
	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/shared"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeleteCustomResourceByName(name string, metadata base.Metadata) shared.KubernetesAPIData {
	prep := prepare(name, metadata, constants.CrdDeleteTimeout)
	if !prep.ok {
		return prep.errEnvelope
	}
	defer prep.cancel()

	if err := prep.client.Delete(prep.ctx, name, k8smetav1.DeleteOptions{}); err != nil {
		return errorEnvelope(errors.ErrDeleteRes, name, err)
	}
	return okEnvelope(messages.SuccessDeleteRes, name, metadata.Kind, nil)
}
