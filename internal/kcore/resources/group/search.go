package group

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/telark/kcore/constants"
	"github.com/telark/kcore/k8sclient"
	"github.com/telark/kcore/resilience/timeout"
	"github.com/telark/kcore/resources/core"
	"github.com/telark/kcore/shared"
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
	Owners    []OwnerRef        `json:"owners,omitempty"`
	// ConfigMaps and Secrets a workload reads; empty for other kinds.
	ConfigMapRefs []string `json:"configMapRefs,omitempty"`
	SecretRefs    []string `json:"secretRefs,omitempty"`
}

type OwnerRef struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type SearchInput struct {
	LabelSelector string
	SearchText    string
	UseSelector   bool
}

const (
	metadataField = "metadata"
)

func ParseSearch(input string) (SearchInput, error) {
	s := strings.TrimSpace(input)
	if s == constants.EmptyString {
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
	if len(namespaces) == constants.EmptySliceLength {
		var listErr error
		namespaces, listErr = listNamespaceNames()
		if listErr != nil {
			return nil, listErr
		}
	}
	ctx, cancel := timeout.ContextWithTimeoutCause(constants.GroupSearchTimeout)
	defer cancel()
	return listAllInNamespaces(ctx, dyn, namespaces, shared.AppGVRs()), nil
}

type listJobState struct {
	sem chan struct{}
	wg  *sync.WaitGroup
	mu  *sync.Mutex
	out *[]ResourceRef
}

func listAllInNamespaces(
	ctx context.Context,
	dyn dynamic.Interface,
	namespaces []string,
	gvrs []schema.GroupVersionResource,
) []ResourceRef {
	var (
		mu  sync.Mutex
		out []ResourceRef
		wg  sync.WaitGroup
	)
	state := &listJobState{
		sem: make(chan struct{}, constants.GroupSearchMaxConcurrent),
		wg:  &wg,
		mu:  &mu,
		out: &out,
	}
	opts := k8smetav1.ListOptions{}
	for _, ns := range namespaces {
		for _, gvr := range gvrs {
			wg.Add(constants.WorkerPoolAddCount)
			go runListJob(ctx, dyn, ns, gvr, opts, state)
		}
	}
	wg.Wait()
	return out
}

func runListJob(
	ctx context.Context,
	dyn dynamic.Interface,
	ns string,
	gvr schema.GroupVersionResource,
	opts k8smetav1.ListOptions,
	state *listJobState,
) {
	defer state.wg.Done()
	if !acquireListSlot(ctx, state.sem) {
		return
	}
	defer func() { <-state.sem }()
	refs := listOneGVRInNamespace(ctx, dyn, ns, gvr, opts)
	if len(refs) == constants.EmptySliceLength {
		return
	}
	state.mu.Lock()
	*state.out = append(*state.out, refs...)
	state.mu.Unlock()
}

func acquireListSlot(ctx context.Context, sem chan struct{}) bool {
	select {
	case sem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func listOneGVRInNamespace(
	ctx context.Context,
	dyn dynamic.Interface,
	ns string,
	gvr schema.GroupVersionResource,
	opts k8smetav1.ListOptions,
) []ResourceRef {
	list, err := dyn.Resource(gvr).Namespace(ns).List(ctx, opts)
	if err != nil {
		return nil
	}
	kind := shared.ResourceKind(gvr.Resource)
	refs := make([]ResourceRef, constants.EmptySliceLength, len(list.Items))
	for i := range list.Items {
		refs = append(refs, toRef(&list.Items[i], ns, kind))
	}
	return refs
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
	if len(namespaces) == constants.EmptySliceLength {
		var listErr error
		namespaces, listErr = listNamespaceNames()
		if listErr != nil {
			return nil, listErr
		}
	}
	ctx, cancel := timeout.ContextWithTimeoutCause(constants.GroupSearchTimeout)
	defer cancel()
	return collectMatching(ctx, dyn, namespaces, shared.AppGVRs(), in), nil
}

func listNamespaceNames() ([]string, error) {
	nsList, err := core.GetAllNamespaces()
	if err != nil {
		return nil, err
	}
	names := make([]string, constants.EmptySliceLength, len(nsList))
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
) []ResourceRef {
	var out []ResourceRef
	listOpts := k8smetav1.ListOptions{}
	if in.UseSelector {
		listOpts.LabelSelector = in.LabelSelector
	}
	for _, ns := range namespaces {
		for _, gvr := range gvrs {
			refs, err := listGVRInNamespace(ctx, dyn, gvr, ns, listOpts, in.SearchText)
			if err != nil {
				continue
			}
			out = append(out, refs...)
		}
	}
	return out
}

func listGVRInNamespace(
	ctx context.Context,
	dyn dynamic.Interface,
	gvr schema.GroupVersionResource,
	namespace string,
	opts k8smetav1.ListOptions,
	searchText string,
) ([]ResourceRef, error) {
	list, err := dyn.Resource(gvr).Namespace(namespace).List(ctx, opts)
	if err != nil {
		return nil, err
	}
	kind := shared.ResourceKind(gvr.Resource)
	refs := make([]ResourceRef, constants.EmptySliceLength, len(list.Items))
	usedSelector := opts.LabelSelector != constants.EmptyString
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
	lbls, _, _ := unstructured.NestedStringMap(u.Object, metadataField, "labels")
	name, _, _ := unstructured.NestedString(u.Object, metadataField, "name")
	if name == constants.EmptyString {
		name = u.GetName()
	}
	cms, secs := WorkloadConfigRefs(u)
	return ResourceRef{
		Namespace: namespace, Kind: kind, Name: name, Labels: lbls, Owners: OwnersOf(u),
		ConfigMapRefs: cms, SecretRefs: secs,
	}
}

// OwnersOf keeps the owner kinds and names so derivation can follow them.
func OwnersOf(u *unstructured.Unstructured) []OwnerRef {
	refs := u.GetOwnerReferences()
	if len(refs) == constants.EmptySliceLength {
		return nil
	}
	out := make([]OwnerRef, constants.EmptySliceLength, len(refs))
	for i := range refs {
		out = append(out, OwnerRef{Kind: refs[i].Kind, Name: refs[i].Name})
	}
	return out
}

func labelsMatchText(u *unstructured.Unstructured, text string) bool {
	lbls, _, _ := unstructured.NestedStringMap(u.Object, metadataField, "labels")
	lower := strings.ToLower(text)
	for k, v := range lbls {
		if strings.Contains(strings.ToLower(k), lower) || strings.Contains(strings.ToLower(v), lower) {
			return true
		}
	}
	return false
}
