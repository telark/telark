package api

import (
	"github.com/plsyro/data/errors"
	"github.com/plsyro/data/messages"
	"github.com/plsyro/kcore/admissions/utils"
	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/resilience/timeout"
	"github.com/plsyro/kcore/shared"
	k8sadmissionsv1 "k8s.io/api/admissionregistration/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateValidatingAdmissionByConfig(
	webhookConfig *k8sadmissionsv1.ValidatingWebhookConfiguration,
) shared.KubernetesAPIData {
	client, err := utils.GetClient()
	if err != nil {
		return shared.HandleClientError(err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.AdmissionCreateTimeout)
	defer cancel()

	operation := func() (any, error) {
		return client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Create(
			ctx, webhookConfig, k8smetav1.CreateOptions{})
	}

	return utils.ExecuteWebhookOperation(
		operation,
		string(messages.SuccessCreateValidatingAdmission),
		string(errors.ErrCreateValidatingAdmission),
	)
}

func CreateMutatingAdmissionByConfig(
	webhookConfig *k8sadmissionsv1.MutatingWebhookConfiguration,
) shared.KubernetesAPIData {
	client, err := utils.GetClient()
	if err != nil {
		return shared.HandleClientError(err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.AdmissionCreateTimeout)
	defer cancel()

	operation := func() (any, error) {
		return client.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(
			ctx, webhookConfig, k8smetav1.CreateOptions{})
	}

	return utils.ExecuteWebhookOperation(
		operation,
		string(messages.SuccessCreateMutatingAdmission),
		string(errors.ErrCreateMutatingAdmission),
	)
}
