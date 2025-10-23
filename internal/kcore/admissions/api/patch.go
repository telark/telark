package api

import (
	"encoding/json"

	admissionshared "github.com/plsyro/data/admissions/shared"
	"github.com/plsyro/data/errors"
	"github.com/plsyro/data/messages"
	"github.com/plsyro/kcore/admissions/utils"
	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/resilience/timeout"
	"github.com/plsyro/kcore/shared"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func PatchAdmissionWebhookAnnotationsByName(
	name string,
	webhookType admissionshared.WebhookType,
	annotations map[string]string,
) shared.KubernetesAPIData {
	client, err := utils.GetClient()
	if err != nil {
		return shared.HandleClientError(err)
	}

	payload := utils.CreateAnnotationsPayload(annotations)
	patchBytes, err := json.Marshal(payload)
	if err != nil {
		return shared.CreateKubernetesAPIData(
			shared.StatusInternalServerError,
			string(errors.ErrRestMarshalPayload),
			nil, err)
	}

	ctx, cancel := timeout.ContextWithTimeoutCause(constants.AdmissionPatchTimeout)
	defer cancel()

	validatingOperation := func() (any, error) {
		patchedWebhook, err := client.AdmissionregistrationV1().ValidatingWebhookConfigurations().
			Patch(ctx, name, types.MergePatchType, patchBytes, k8smetav1.PatchOptions{})
		if err != nil {
			return nil, err
		}

		msg := utils.CreateAdmissionMessage(messages.SuccessUpdateAdmission, name, admissionshared.Validating)
		return shared.CreateKubernetesAPIData(shared.StatusOK, msg, patchedWebhook, nil), nil
	}

	mutatingOperation := func() (any, error) {
		patchedWebhook, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().
			Patch(ctx, name, types.MergePatchType, patchBytes, k8smetav1.PatchOptions{})
		if err != nil {
			return nil, err
		}

		msg := utils.CreateAdmissionMessage(messages.SuccessUpdateAdmission, name, admissionshared.Mutating)
		return shared.CreateKubernetesAPIData(shared.StatusOK, msg, patchedWebhook, nil), nil
	}

	return utils.HandleWebhookType(webhookType, validatingOperation, mutatingOperation)
}
