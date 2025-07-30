package batch

import (
	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	k8sClient "github.com/plsyro/kcore-pkg/resources/client"

	batch "k8s.io/api/batch/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type JobAdapter struct{}

func (jobAdapter *JobAdapter) GetJobsByNamespace(namespace string) ([]batch.Job, error) {
	client, err := k8sClient.InitClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.WORKLOAD_LIST_TIMEOUT)
	defer cancel()

	jobs, err := client.BatchV1().Jobs(namespace).List(ctx, meta.ListOptions{})
	if err != nil {
		return nil, err
	}
	return jobs.Items, nil
}
