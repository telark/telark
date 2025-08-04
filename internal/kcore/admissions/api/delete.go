package api

import (
	admissionShared "github.com/plsyro/data-pkg/admissions/shared"
	"github.com/plsyro/data-pkg/messages"
	"github.com/plsyro/kcore-pkg/admissions/utils"
	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	"github.com/plsyro/kcore-pkg/shared"
	kubeApiMeta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeleteAdmissionWebhookByName(name string, webhookType admissionShared.WebhookType) shared.KubernetesAPIData {
	client, err := utils.GetClient()
	if err != nil {
		return shared.HandleClientError(err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.AdmissionDeleteTimeout)
	defer cancel()

	validatingOperation := func() (interface{}, error) {
		err := client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Delete(ctx, name, kubeApiMeta.DeleteOptions{})
		if err != nil {
			return nil, err
		}
		return shared.CreateKubernetesAPIData(shared.StatusOK, string(messages.SuccessDeleteValidatingAdmission), nil, nil), nil
	}

	mutatingOperation := func() (interface{}, error) {
		err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(ctx, name, kubeApiMeta.DeleteOptions{})
		if err != nil {
			return nil, err
		}
		return shared.CreateKubernetesAPIData(shared.StatusOK, string(messages.SuccessDeleteMutatingAdmission), nil, nil), nil
	}

	return utils.HandleWebhookType(webhookType, validatingOperation, mutatingOperation)
}
