package batch

import (
	"fmt"

	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/k8sclient"
	"github.com/plsyro/kcore/resilience/timeout"
	batch "k8s.io/api/batch/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetJobsByNamespace(namespace string) ([]batch.Job, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeout.ContextWithTimeoutCause(constants.WorkloadListTimeout)
	defer cancel()

	jobs, err := client.BatchV1().Jobs(namespace).List(ctx, meta.ListOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToFetchJobs), namespace, err))
		return nil, err
	}
	return jobs.Items, nil
}
