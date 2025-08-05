package api

import (
	admissionShared "github.com/plsyro/data/admissions/shared"
	"github.com/plsyro/data/messages"
	"github.com/plsyro/kcore/admissions/utils"
	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/resilience/timeout"
	"github.com/plsyro/kcore/shared"
	kubeApiMeta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetAdmissionWebhookByName(name string, webhookType admissionShared.WebhookType) shared.KubernetesAPIData {
	client, err := utils.GetClient()
	if err != nil {
		return shared.HandleClientError(err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.AdmissionGetTimeout)
	defer cancel()

	validatingOperation := func() (interface{}, error) {
		return client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(ctx, name, kubeApiMeta.GetOptions{})
	}

	mutatingOperation := func() (interface{}, error) {
		return client.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, name, kubeApiMeta.GetOptions{})
	}

	validatingHandler := func() (interface{}, error) {
		result, err := validatingOperation()
		if err != nil {
			return nil, err
		}
		return shared.CreateKubernetesAPIData(shared.StatusOK, string(messages.SuccessGetValidatingAdmission), result, nil), nil
	}

	mutatingHandler := func() (interface{}, error) {
		result, err := mutatingOperation()
		if err != nil {
			return nil, err
		}
		return shared.CreateKubernetesAPIData(shared.StatusOK, string(messages.SuccessGetMutatingAdmission), result, nil), nil
	}

	return utils.HandleWebhookType(webhookType, validatingHandler, mutatingHandler)
}
