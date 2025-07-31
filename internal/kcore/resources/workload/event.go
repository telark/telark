package workload

import (
	"fmt"

	workload "github.com/plsyro/data-pkg/resources/workload/common"
	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/k8sclient"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetAllEventsByPod(pod, namespace string, selectors map[string]string) ([]workload.EventItem, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, err
	}

	eventsPerPod := make([]workload.EventItem, 0)

	ctx, cancel := timeout.ContextWithTimeout(constants.EVENT_FETCH_TIMEOUT)
	defer cancel()

	events, err := client.CoreV1().Events(namespace).List(ctx, meta.ListOptions{
		FieldSelector: fmt.Sprintf(constants.FIELD_SELECTOR_INVOLVED_OBJECT, pod),
	})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ERROR_FAILED_TO_FETCH_POD_EVENTS), pod, namespace, err))
		return eventsPerPod, err
	}

	for _, e := range events.Items {
		eventPerPod := workload.EventItem{
			Type:    e.Type,
			Reason:  e.Reason,
			Message: e.Message,
		}
		eventsPerPod = append(eventsPerPod, eventPerPod)
	}
	return eventsPerPod, nil
}
