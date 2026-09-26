package workloads

import (
	"fmt"
	"net/http"

	"github.com/telark/discovery/internal/constants"
	analyzeshared "github.com/telark/discovery/internal/handlers/analyze/shared"
	sharedhelper "github.com/telark/discovery/internal/helpers/shared"
	"github.com/telark/kcore/resources/workload"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
)

type NamespaceWorkloadsResponse struct {
	Deployments  []string `json:"deployments"`
	StatefulSets []string `json:"statefulSets"`
	DaemonSets   []string `json:"daemonSets"`
	Jobs         []string `json:"jobs"`
	CronJobs     []string `json:"cronJobs"`
}

func ListNamespaceWorkloads(w http.ResponseWriter, r *http.Request) {
	namespace, err := sharedhelper.GetPathParam(w, r, constants.NamespaceParam)
	if err != nil || !analyzeshared.NamespaceListable(w, r, namespace) {
		return
	}

	result, err := listWorkloadNames(namespace)
	if err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationError,
			fmt.Sprintf(string(constants.ErrFailedListNamespaceWorkloads), namespace, err),
			nil,
			err,
		)
		return
	}

	response.SendSingleResponse(
		w,
		response.NewGenericResponse(
			http.StatusOK,
			response.OperationSuccess,
			result,
			string(constants.SuccessWorkloadsListed),
		),
	)
}

func listWorkloadNames(namespace string) (*NamespaceWorkloadsResponse, error) {
	resp := &NamespaceWorkloadsResponse{
		Deployments:  []string{},
		StatefulSets: []string{},
		DaemonSets:   []string{},
		Jobs:         []string{},
		CronJobs:     []string{},
	}

	fetchers := []analyzeshared.NameFetcher{
		{Dst: &resp.Deployments, Fn: func(ns string) ([]string, error) {
			return analyzeshared.NamesFromList(ns, workload.GetDeploymentsByNamespace,
				func(r appsv1.Deployment) string { return r.Name })
		}},
		{Dst: &resp.StatefulSets, Fn: func(ns string) ([]string, error) {
			return analyzeshared.NamesFromList(ns, workload.GetStatefulSetsByNamespace,
				func(r appsv1.StatefulSet) string { return r.Name })
		}},
		{Dst: &resp.DaemonSets, Fn: func(ns string) ([]string, error) {
			return analyzeshared.NamesFromList(ns, workload.GetDaemonSetsByNamespace,
				func(r appsv1.DaemonSet) string { return r.Name })
		}},
		{Dst: &resp.Jobs, Fn: func(ns string) ([]string, error) {
			return analyzeshared.NamesFromList(ns, workload.GetJobsByNamespace,
				func(r batchv1.Job) string { return r.Name })
		}},
		{Dst: &resp.CronJobs, Fn: func(ns string) ([]string, error) {
			return analyzeshared.NamesFromList(ns, workload.GetCronJobsByNamespace,
				func(r batchv1.CronJob) string { return r.Name })
		}},
	}

	if err := analyzeshared.RunFetchers(namespace, fetchers); err != nil {
		return nil, err
	}
	return resp, nil
}
