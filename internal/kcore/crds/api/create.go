package api

import (
	"fmt"

	"github.com/plsyro/data/errors"
	"github.com/plsyro/data/messages"
	"github.com/plsyro/data/metadata/base"
	"github.com/plsyro/kcore/constants"
	crdutils "github.com/plsyro/kcore/crds/utils"
	"github.com/plsyro/kcore/resilience/timeout"
	"github.com/plsyro/kcore/shared"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func CreateCustomResource(template *unstructured.Unstructured, metadata base.Metadata) shared.KubernetesAPIData {
	resourceClient, err := crdutils.GetResourceClient(metadata)
	if err != nil {
		return shared.HandleClientError(err)
	}

	ctx, cancel := timeout.ContextWithTimeoutCause(constants.CrdCreateTimeout)
	defer cancel()

	resource, err := resourceClient.Create(ctx, template, k8smetav1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			return shared.CreateKubernetesAPIData(shared.StatusConflict, string(errors.ErrResExists), nil, nil)
		}
		message := fmt.Sprintf(string(errors.ErrCreateRes), template.GetName(), err)
		return shared.CreateKubernetesAPIData(shared.StatusInternalServerError, message, nil, err)
	}

	message := fmt.Sprintf(string(messages.SuccessCreateRes), template.GetName(), metadata.Kind)
	return shared.CreateKubernetesAPIData(shared.StatusOK, message, resource, nil)
}
