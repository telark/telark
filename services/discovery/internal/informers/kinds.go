package informers

import (
	"slices"

	applicationmodel "github.com/telark/data/resources/application"
	"github.com/telark/discovery/constants"
	kcoreshared "github.com/telark/kcore/shared"
)

func kindSetFromApps(apps []applicationmodel.Application) map[string]struct{} {
	out := make(map[string]struct{})
	for i := range apps {
		for _, r := range apps[i].Resources {
			if r.Kind != constants.EmptyString {
				out[r.Kind] = struct{}{}
			}
		}
	}
	if len(out) == constants.DefaultInitValue {
		for _, gvr := range kcoreshared.AppGVRs() {
			k := kcoreshared.ResourceKind(gvr.Resource)
			if k != constants.EmptyString && !informerExcludedKind(k) {
				out[k] = struct{}{}
			}
		}
	}
	for _, k := range constants.InformerExcludedKinds {
		delete(out, k)
	}
	return out
}

func informerExcludedKind(kind string) bool {
	return slices.Contains(constants.InformerExcludedKinds, kind)
}
