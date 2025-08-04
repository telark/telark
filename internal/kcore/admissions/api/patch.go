package api

import (
	"encoding/json"

	"github.com/plsyro/data-pkg/admissions/common"
	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/data-pkg/messages"
	"github.com/plsyro/kcore-pkg/admissions/utils"
	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	"github.com/plsyro/kcore-pkg/shared"
	kubeApiMeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func PatchAdmissionWebhookAnnotationsByName(name string, webhookType common.WebhookType, annotations map[string]string) shared.KubernetesAPIData {
	client, err := utils.GetClient()
	if err != nil {
		return shared.HandleClientError(err)
	}

	payload := utils.CreateAnnotationsPayload(annotations)
	patchBytes, err := json.Marshal(payload)
	if err != nil {
		return shared.CreateKubernetesAPIData(shared.StatusInternalServerError, string(errors.ERROR_REST_MARSHALL_PAYLOAD), nil, err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.AdmissionPatchTimeout)
	defer cancel()

	validatingOperation := func() (interface{}, error) {
		patchedWebhook, err := client.AdmissionregistrationV1().ValidatingWebhookConfigurations().
			Patch(ctx, name, types.MergePatchType, patchBytes, kubeApiMeta.PatchOptions{})
		if err != nil {
			return nil, err
		}
		message := utils.CreateMessage(patchedWebhook.GetName(), string(common.VALIDATING), string(messages.SUCCESS_UPDATE_RESOURCE))
		return shared.CreateKubernetesAPIData(shared.StatusOK, message, patchedWebhook, nil), nil
	}

	mutatingOperation := func() (interface{}, error) {
		patchedWebhook, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().
			Patch(ctx, name, types.MergePatchType, patchBytes, kubeApiMeta.PatchOptions{})
		if err != nil {
			return nil, err
		}
		message := utils.CreateMessage(patchedWebhook.GetName(), string(common.MUTATING), string(messages.SUCCESS_UPDATE_RESOURCE))
		return shared.CreateKubernetesAPIData(shared.StatusOK, message, patchedWebhook, nil), nil
	}

	return utils.HandleWebhookType(webhookType, validatingOperation, mutatingOperation)
}
