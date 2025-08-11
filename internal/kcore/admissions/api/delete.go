package api

import (
	admissionshared "github.com/plsyro/data/admissions/shared"
	"github.com/plsyro/data/messages"
	"github.com/plsyro/kcore/admissions/utils"
	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/resilience/timeout"
	"github.com/plsyro/kcore/shared"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeleteAdmissionWebhookByName(name string, webhookType admissionshared.WebhookType) shared.KubernetesAPIData {
	client, err := utils.GetClient()
	if err != nil {
		return shared.HandleClientError(err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.AdmissionDeleteTimeout)
	defer cancel()

	validatingOperation := func() (any, error) {
		err := client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Delete(
			ctx, name, k8smetav1.DeleteOptions{})
		if err != nil {
			return nil, err
		}

		return shared.CreateKubernetesAPIData(
			shared.StatusOK,
			utils.CreateAdmissionMessage(messages.SuccessDeleteAdmission, name, admissionshared.Validating),
			nil,
			nil,
		), nil
	}

	mutatingOperation := func() (any, error) {
		err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(
			ctx, name, k8smetav1.DeleteOptions{})
		if err != nil {
			return nil, err
		}

		return shared.CreateKubernetesAPIData(
			shared.StatusOK,
			utils.CreateAdmissionMessage(messages.SuccessDeleteAdmission, name, admissionshared.Mutating),
			nil,
			nil,
		), nil
	}

	return utils.HandleWebhookType(webhookType, validatingOperation, mutatingOperation)
}
