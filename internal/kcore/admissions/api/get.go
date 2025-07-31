package api

import (
	"github.com/plsyro/data-pkg/admissions/common"
	"github.com/plsyro/data-pkg/messages"
	"github.com/plsyro/kcore-pkg/admissions/utils"
	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	kubeApiMeta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetAdmissionWebhookByName(name string, webhookType common.WebhookType) utils.AdmissionWebhookData {
	client, err := utils.GetClient()
	if err != nil {
		return utils.HandleClientError(err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.ADMISSION_GET_TIMEOUT)
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
		return utils.CreateAdmissionWebhookData(utils.StatusOK, string(messages.SUCCESS_GET_VALIDATING_ADMISSION), result, nil), nil
	}

	mutatingHandler := func() (interface{}, error) {
		result, err := mutatingOperation()
		if err != nil {
			return nil, err
		}
		return utils.CreateAdmissionWebhookData(utils.StatusOK, string(messages.SUCCESS_GET_MUTATING_ADMISSION), result, nil), nil
	}

	return utils.HandleWebhookType(webhookType, validatingHandler, mutatingHandler)
}
