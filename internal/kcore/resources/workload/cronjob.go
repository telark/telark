package workload

import (
	"fmt"

	"github.com/telark/kcore/constants"
	"github.com/telark/kcore/k8sclient"
	"github.com/telark/kcore/resilience/timeout"
	k8sbatchv1 "k8s.io/api/batch/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetCronJobsByNamespace(namespace string) ([]k8sbatchv1.CronJob, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeout.ContextWithTimeoutCause(constants.WorkloadListTimeout)
	defer cancel()

	cronjobs, err := client.BatchV1().CronJobs(namespace).List(ctx, k8smetav1.ListOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToFetchJobs), namespace, err))
		return nil, err
	}
	return cronjobs.Items, nil
}
