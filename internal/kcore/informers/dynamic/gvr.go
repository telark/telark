package dynamic

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

func GVRsForKinds(kindSet map[string]struct{}, disco discovery.DiscoveryInterface) ([]schema.GroupVersionResource, error) {
	if len(kindSet) == 0 {
		return nil, nil
	}
	grs, err := disco.ServerPreferredResources()
	if err != nil {
		return nil, err
	}
	out := make([]schema.GroupVersionResource, 0, len(kindSet))
	seen := make(map[schema.GroupVersionResource]struct{})
	for _, list := range grs {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}
		for i := range list.APIResources {
			r := &list.APIResources[i]
			if !apiResourceWatchable(r) {
				continue
			}
			if _, want := kindSet[r.Kind]; !want {
				continue
			}
			gvr := gv.WithResource(r.Name)
			if _, dup := seen[gvr]; dup {
				continue
			}
			seen[gvr] = struct{}{}
			out = append(out, gvr)
		}
	}
	return out, nil
}

func apiResourceWatchable(r *metav1.APIResource) bool {
	if r == nil {
		return false
	}
	if !r.Namespaced {
		return false
	}
	for _, v := range r.Verbs {
		if v == "list" || v == "watch" {
			return true
		}
	}
	return false
}
