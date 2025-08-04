package api

import (
	"github.com/plsyro/data-pkg/admissions/common"
	"github.com/plsyro/data-pkg/messages"
	webhookutils "github.com/plsyro/kcore-pkg/admissions/utils"
	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	"github.com/plsyro/kcore-pkg/shared"
	kubeApiMeta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeleteAdmissionWebhookByName(name string, webhookType common.WebhookType) shared.KubernetesAPIData {
	client, err := webhookutils.GetClient()
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
		return shared.CreateKubernetesAPIData(shared.StatusOK, string(messages.SUCCESS_DELETE_VALIDATING_ADMISSION), nil, nil), nil
	}

	mutatingOperation := func() (interface{}, error) {
		err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(ctx, name, kubeApiMeta.DeleteOptions{})
		if err != nil {
			return nil, err
		}
		return shared.CreateKubernetesAPIData(shared.StatusOK, string(messages.SUCCESS_DELETE_MUTATING_ADMISSION), nil, nil), nil
	}

	return webhookutils.HandleWebhookType(webhookType, validatingOperation, mutatingOperation)
}
