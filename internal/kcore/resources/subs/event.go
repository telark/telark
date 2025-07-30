package subs

import (
	"fmt"

	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	"github.com/plsyro/kcore-pkg/resources/client"

	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/data-pkg/logging"
	workload "github.com/plsyro/data-pkg/resources/workload/common"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var logger = logging.NewCustomLogger(constants.LOGGER_PREFIX_EVENT)

type EventAdapter struct{}

func (eventAdapter *EventAdapter) FetchEventsByPod(pod string, namespace string, selectors map[string]string) []workload.EventItem {
	client, err := client.InitClient()
	if err != nil {
		return nil
	}

	eventsPerPod := make([]workload.EventItem, 0)

	ctx, cancel := timeout.ContextWithTimeout(constants.EVENT_FETCH_TIMEOUT)
	defer cancel()

	events, err := client.CoreV1().Events(namespace).List(ctx, meta.ListOptions{
		FieldSelector: fmt.Sprintf(constants.FIELD_SELECTOR_INVOLVED_OBJECT, pod),
	})
	if err != nil {
		logger.Error(fmt.Sprintf(string(errors.ERROR_K8S_FETCHING_POD_EVENTS), pod, namespace, err))
		return eventsPerPod
	}

	for _, e := range events.Items {
		eventPerPod := workload.EventItem{
			Type:    e.Type,
			Reason:  e.Reason,
			Message: e.Message,
		}
		eventsPerPod = append(eventsPerPod, eventPerPod)
	}
	return eventsPerPod
}
