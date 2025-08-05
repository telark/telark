package api

import (
	"fmt"

	admissionshared "github.com/plsyro/data/admissions/shared"
	"github.com/plsyro/kcore/admissions/utils"
	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/resilience/timeout"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CheckAdmissionWebhookExistsByName(
	name string,
	webhookType admissionshared.WebhookType,
) (bool, error) {
	client, err := utils.GetClient()
	if err != nil {
		return false, err
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.AdmissionGetTimeout)
	defer cancel()

	switch webhookType {
	case admissionshared.Validating:
		_, err := client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
			ctx, name, k8smetav1.GetOptions{})
		if err == nil {
			return true, nil
		}
		return false, err

	case admissionshared.Mutating:
		_, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(
			ctx, name, k8smetav1.GetOptions{})
		if err == nil {
			return true, nil
		}
		return false, err

	default:
		return false, fmt.Errorf("%s", constants.ErrInvalidWebhookType)
	}
}
