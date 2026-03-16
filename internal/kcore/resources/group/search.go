package group

import (
	"context"
	"fmt"
	"strings"

	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/k8sclient"
	"github.com/plsyro/kcore/resilience/timeout"
	"github.com/plsyro/kcore/resources/core"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type ResourceRef struct {
	Namespace string            `json:"namespace"`
	Kind      string            `json:"kind"`
	Name      string            `json:"name"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type SearchInput struct {
	LabelSelector string
	SearchText    string
	UseSelector   bool
}

var resourceToKind = map[string]string{
	"deployments": "Deployment", "statefulsets": "StatefulSet", "daemonsets": "DaemonSet",
	"jobs": "Job", "cronjobs": "CronJob", "configmaps": "ConfigMap", "secrets": "Secret",
	"services": "Service", "persistentvolumeclaims": "PersistentVolumeClaim",
	"serviceaccounts": "ServiceAccount", "ingresses": "Ingress",
	"networkpolicies": "NetworkPolicy", "horizontalpodautoscalers": "HorizontalPodAutoscaler",
	"verticalpodautoscalers": "VerticalPodAutoscaler",
}

func resourceKind(resource string) string {
	if k, ok := resourceToKind[resource]; ok {
		return k
	}
	return resource
}

func AppGVRs() []schema.GroupVersionResource {
	return []schema.GroupVersionResource{
		{Group: "apps", Version: "v1", Resource: "deployments"},
		{Group: "apps", Version: "v1", Resource: "statefulsets"},
		{Group: "apps", Version: "v1", Resource: "daemonsets"},
		{Group: "batch", Version: "v1", Resource: "jobs"},
		{Group: "batch", Version: "v1", Resource: "cronjobs"},
		{Group: "", Version: "v1", Resource: "configmaps"},
		{Group: "", Version: "v1", Resource: "secrets"},
		{Group: "", Version: "v1", Resource: "services"},
		{Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
		{Group: "", Version: "v1", Resource: "serviceaccounts"},
		{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
		{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"},
		{Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"},
		{Group: "autoscaling.k8s.io", Version: "v1", Resource: "verticalpodautoscalers"},
	}
}

func ParseSearch(input string) (SearchInput, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return SearchInput{}, fmt.Errorf("%s", constants.ErrEmptySearchParam)
	}
	selectorStr := strings.ReplaceAll(s, ":", "=")
	if strings.Contains(selectorStr, "=") {
		if _, err := labels.Parse(selectorStr); err != nil {
			return SearchInput{SearchText: s, UseSelector: false}, nil
		}
		return SearchInput{LabelSelector: selectorStr, UseSelector: true}, nil
	}
	return SearchInput{SearchText: s, UseSelector: false}, nil
}

func SearchResourcesByLabelOrText(search string) ([]ResourceRef, error) {
	return SearchResourcesByLabelOrTextInNamespaces(search, nil)
}

func ListAllResourcesInNamespaces(namespaces []string) ([]ResourceRef, error) {
	dyn, err := k8sclient.InitDynamicClient()
	if err != nil {
		return nil, err
	}
	if len(namespaces) == 0 {
		var listErr error
		namespaces, listErr = listNamespaceNames()
		if listErr != nil {
			return nil, listErr
		}
	}
	ctx, cancel := timeout.ContextWithTimeoutCause(constants.GroupSearchTimeout)
	defer cancel()
	return listAllInNamespaces(ctx, dyn, namespaces, AppGVRs())
}

func listAllInNamespaces(
	ctx context.Context,
	dyn dynamic.Interface,
	namespaces []string,
	gvrs []schema.GroupVersionResource,
) ([]ResourceRef, error) {
	var out []ResourceRef
	opts := k8smetav1.ListOptions{}
	for _, ns := range namespaces {
		for _, gvr := range gvrs {
			list, err := dyn.Resource(gvr).Namespace(ns).List(ctx, opts)
			if err != nil {
				continue
			}
			kind := resourceKind(gvr.Resource)
			for i := range list.Items {
				out = append(out, toRef(&list.Items[i], ns, kind))
			}
		}
	}
	return out, nil
}

func SearchResourcesByLabelOrTextInNamespaces(search string, namespaces []string) ([]ResourceRef, error) {
	in, err := ParseSearch(search)
	if err != nil {
		return nil, err
	}
	dyn, err := k8sclient.InitDynamicClient()
	if err != nil {
		return nil, err
	}
	if len(namespaces) == 0 {
		var listErr error
		namespaces, listErr = listNamespaceNames()
		if listErr != nil {
			return nil, listErr
		}
	}
	ctx, cancel := timeout.ContextWithTimeoutCause(constants.GroupSearchTimeout)
	defer cancel()
	return collectMatching(ctx, dyn, namespaces, AppGVRs(), in)
}

func listNamespaceNames() ([]string, error) {
	nsList, err := core.GetAllNamespaces()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(nsList))
	for i := range nsList {
		names = append(names, nsList[i].Name)
	}
	return names, nil
}

func collectMatching(
	ctx context.Context,
	dyn dynamic.Interface,
	namespaces []string,
	gvrs []schema.GroupVersionResource,
	in SearchInput,
) ([]ResourceRef, error) {
	var out []ResourceRef
	listOpts := k8smetav1.ListOptions{}
	if in.UseSelector {
		listOpts.LabelSelector = in.LabelSelector
	}
	for _, ns := range namespaces {
		for _, gvr := range gvrs {
			refs, err := listGVRInNamespace(ctx, dyn, gvr, ns, listOpts, in.SearchText, in.UseSelector)
			if err != nil {
				continue
			}
			out = append(out, refs...)
		}
	}
	return out, nil
}

func listGVRInNamespace(
	ctx context.Context,
	dyn dynamic.Interface,
	gvr schema.GroupVersionResource,
	namespace string,
	opts k8smetav1.ListOptions,
	searchText string,
	usedSelector bool,
) ([]ResourceRef, error) {
	list, err := dyn.Resource(gvr).Namespace(namespace).List(ctx, opts)
	if err != nil {
		return nil, err
	}
	kind := resourceKind(gvr.Resource)
	refs := make([]ResourceRef, 0, len(list.Items))
	for i := range list.Items {
		item := &list.Items[i]
		if usedSelector {
			refs = append(refs, toRef(item, namespace, kind))
			continue
		}
		if labelsMatchText(item, searchText) {
			refs = append(refs, toRef(item, namespace, kind))
		}
	}
	return refs, nil
}

func toRef(u *unstructured.Unstructured, namespace, kind string) ResourceRef {
	lbls, _, _ := unstructured.NestedStringMap(u.Object, "metadata", "labels")
	name, _, _ := unstructured.NestedString(u.Object, "metadata", "name")
	if name == "" {
		name = u.GetName()
	}
	return ResourceRef{Namespace: namespace, Kind: kind, Name: name, Labels: lbls}
}

func labelsMatchText(u *unstructured.Unstructured, text string) bool {
	lbls, _, _ := unstructured.NestedStringMap(u.Object, "metadata", "labels")
	lower := strings.ToLower(text)
	for k, v := range lbls {
		if strings.Contains(strings.ToLower(k), lower) || strings.Contains(strings.ToLower(v), lower) {
			return true
		}
	}
	return false
}
