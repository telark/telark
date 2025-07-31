package api

import (
	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/data-pkg/messages"
	"github.com/plsyro/kcore-pkg/admissions/utils"
	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	"github.com/plsyro/kcore-pkg/shared"
	kubeApiAdmissionv1 "k8s.io/api/admissionregistration/v1"
	kubeApiMeta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateValidatingAdmissionByConfig(webhookConfig *kubeApiAdmissionv1.ValidatingWebhookConfiguration) shared.KubernetesAPIData {
	client, err := utils.GetClient()
	if err != nil {
		return shared.HandleClientError(err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.ADMISSION_CREATE_TIMEOUT)
	defer cancel()

	operation := func() (interface{}, error) {
		return client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Create(ctx, webhookConfig, kubeApiMeta.CreateOptions{})
	}

	return utils.ExecuteWebhookOperation(operation, string(messages.SUCCESS_CREATE_VALIDATING_ADMISSION), string(errors.ERROR_CREATE_VALIDATING_ADMISSION))
}

func CreateMutatingAdmissionByConfig(webhookConfig *kubeApiAdmissionv1.MutatingWebhookConfiguration) shared.KubernetesAPIData {
	client, err := utils.GetClient()
	if err != nil {
		return shared.HandleClientError(err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.ADMISSION_CREATE_TIMEOUT)
	defer cancel()

	operation := func() (interface{}, error) {
		return client.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(ctx, webhookConfig, kubeApiMeta.CreateOptions{})
	}

	return utils.ExecuteWebhookOperation(operation, string(messages.SUCCESS_CREATE_MUTATING_ADMISSION), string(errors.ERROR_CREATE_MUTATING_ADMISSION))
}
