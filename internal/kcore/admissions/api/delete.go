package api

import (
	"github.com/plsyro/data-pkg/admissions/common"
	"github.com/plsyro/data-pkg/messages"
	"github.com/plsyro/kcore-pkg/admissions/utils"
	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	kubeApiMeta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeleteAdmissionWebhookByName(name string, webhookType common.WebhookType) utils.AdmissionWebhookData {
	client, err := utils.GetClient()
	if err != nil {
		return utils.HandleClientError(err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.ADMISSION_DELETE_TIMEOUT)
	defer cancel()

	validatingOperation := func() (interface{}, error) {
		err := client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Delete(ctx, name, kubeApiMeta.DeleteOptions{})
		if err != nil {
			return nil, err
		}
		return utils.CreateAdmissionWebhookData(utils.StatusOK, string(messages.SUCCESS_DELETE_VALIDATING_ADMISSION), nil, nil), nil
	}

	mutatingOperation := func() (interface{}, error) {
		err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(ctx, name, kubeApiMeta.DeleteOptions{})
		if err != nil {
			return nil, err
		}
		return utils.CreateAdmissionWebhookData(utils.StatusOK, string(messages.SUCCESS_DELETE_MUTATING_ADMISSION), nil, nil), nil
	}

	return utils.HandleWebhookType(webhookType, validatingOperation, mutatingOperation)
}
