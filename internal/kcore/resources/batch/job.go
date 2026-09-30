package batch

import (
	batch "k8s.io/api/batch/v1"

	"github.com/telark/kcore/resources/workload"
)

func GetJobsByNamespace(namespace string) ([]batch.Job, error) {
	return workload.GetJobsByNamespace(namespace)
}
