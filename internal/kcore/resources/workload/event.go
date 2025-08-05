package workload

import (
	"fmt"

	workload "github.com/plsyro/data/resources/workloads/shared"
	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/k8sclient"
	"github.com/plsyro/kcore/resilience/timeout"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetEventsByPod(pod, namespace string, selectors map[string]string) ([]workload.EventItem, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, err
	}

	eventsPerPod := make([]workload.EventItem, 0)

	ctx, cancel := timeout.ContextWithTimeout(constants.EventFetchTimeout)
	defer cancel()

	events, err := client.CoreV1().Events(namespace).List(ctx, meta.ListOptions{
		FieldSelector: fmt.Sprintf(constants.FieldSelectorInvolvedObject, pod),
	})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToFetchPodEvents), pod, namespace, err))
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
