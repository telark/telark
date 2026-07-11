package resources

import (
	"fmt"
	"net/http"

	"github.com/telark/discovery/constants"
	analyzeshared "github.com/telark/discovery/handlers/analyze/shared"
	sharedhelper "github.com/telark/discovery/helpers/shared"
	"github.com/telark/kcore/resources/autoscaling"
	"github.com/telark/kcore/resources/core"
	"github.com/telark/kcore/resources/networking"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

type NamespaceResourcesResponse struct {
	ConfigMaps               []string `json:"configMaps"`
	Secrets                  []string `json:"secrets"`
	Services                 []string `json:"services"`
	Ingresses                []string `json:"ingresses"`
	NetworkPolicies          []string `json:"networkPolicies"`
	PersistentVolumeClaims   []string `json:"persistentVolumeClaims"`
	HorizontalPodAutoscalers []string `json:"horizontalPodAutoscalers"`
	VerticalPodAutoscalers   []string `json:"verticalPodAutoscalers"`
	ServiceAccounts          []string `json:"serviceAccounts"`
}

func ListNamespaceResources(w http.ResponseWriter, r *http.Request) {
	namespace, err := sharedhelper.GetPathParam(w, r, constants.NamespaceParam)
	if err != nil {
		return
	}

	result, err := listResourceNames(namespace)
	if err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationError,
			fmt.Sprintf(string(constants.ErrFailedListNamespaceResources), namespace, err),
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
			string(constants.SuccessResourcesListed),
		),
	)
}

func listResourceNames(namespace string) (*NamespaceResourcesResponse, error) {
	resp := &NamespaceResourcesResponse{
		ConfigMaps:               []string{},
		Secrets:                  []string{},
		Services:                 []string{},
		Ingresses:                []string{},
		NetworkPolicies:          []string{},
		PersistentVolumeClaims:   []string{},
		HorizontalPodAutoscalers: []string{},
		VerticalPodAutoscalers:   []string{},
		ServiceAccounts:          []string{},
	}

	fetchers := []analyzeshared.NameFetcher{
		{Dst: &resp.ConfigMaps, Fn: func(ns string) ([]string, error) {
			return analyzeshared.NamesFromList(ns, core.GetConfigMapsByNamespace,
				func(r corev1.ConfigMap) string { return r.Name })
		}},
		{Dst: &resp.Secrets, Fn: func(ns string) ([]string, error) {
			return analyzeshared.NamesFromList(ns, core.GetSecretsByNamespace,
				func(r corev1.Secret) string { return r.Name })
		}},
		{Dst: &resp.Services, Fn: func(ns string) ([]string, error) {
			return analyzeshared.NamesFromList(ns, networking.GetServicesByNamespace,
				func(r corev1.Service) string { return r.Name })
		}},
		{Dst: &resp.Ingresses, Fn: func(ns string) ([]string, error) {
			return analyzeshared.NamesFromList(ns, networking.GetIngressesByNamespace,
				func(r networkingv1.Ingress) string { return r.Name })
		}},
		{Dst: &resp.NetworkPolicies, Fn: func(ns string) ([]string, error) {
			return analyzeshared.NamesFromList(ns, networking.GetNetworkPoliciesByNamespace,
				func(r networkingv1.NetworkPolicy) string { return r.Name })
		}},
		{Dst: &resp.PersistentVolumeClaims, Fn: func(ns string) ([]string, error) {
			return analyzeshared.NamesFromList(ns, core.GetPersistentVolumeClaimsByNamespace,
				func(r corev1.PersistentVolumeClaim) string { return r.Name })
		}},
		{Dst: &resp.HorizontalPodAutoscalers, Fn: func(ns string) ([]string, error) {
			return analyzeshared.NamesFromList(ns, autoscaling.GetHorizontalPodAutoscalersByNamespace,
				func(r autoscalingv2.HorizontalPodAutoscaler) string { return r.Name })
		}},
		{Dst: &resp.VerticalPodAutoscalers, Fn: autoscaling.GetVerticalPodAutoscalersByNamespace},
		{Dst: &resp.ServiceAccounts, Fn: func(ns string) ([]string, error) {
			return analyzeshared.NamesFromList(ns, core.GetServiceAccountsByNamespace,
				func(r corev1.ServiceAccount) string { return r.Name })
		}},
	}

	if err := analyzeshared.RunFetchers(namespace, fetchers); err != nil {
		return nil, err
	}
	return resp, nil
}
