package api

import (
	"fmt"

	"github.com/plsyro/data-pkg/admissions/common"
	webhookutils "github.com/plsyro/kcore-pkg/admissions/utils"
	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	kubeApiMeta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CheckAdmissionWebhookExistsByName(name string, webhookType common.WebhookType) (bool, error) {
	client, err := webhookutils.GetClient()
	if err != nil {
		return false, err
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.AdmissionGetTimeout)
	defer cancel()

	switch webhookType {
	case common.VALIDATING:
		_, err := client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(ctx, name, kubeApiMeta.GetOptions{})
		if err == nil {
			return true, nil
		}
		return false, err

	case common.MUTATING:
		_, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, name, kubeApiMeta.GetOptions{})
		if err == nil {
			return true, nil
		}
		return false, err

	default:
		return false, fmt.Errorf("%s", constants.ErrInvalidWebhookType)
	}
}
