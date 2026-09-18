package informers

import (
	"slices"

	"github.com/telark/discovery/internal/constants"
	kcoreshared "github.com/telark/kcore/shared"
)

// watchKindSet is every application kind, not only the kinds current apps
// already use: the informer cache feeds discovery, so a kind that is not
// watched (a first CronJob) could never be discovered.
func watchKindSet() map[string]struct{} {
	out := make(map[string]struct{})
	for _, gvr := range kcoreshared.AppGVRs() {
		k := kcoreshared.ResourceKind(gvr.Resource)
		if k != constants.EmptyString && !informerExcludedKind(k) {
			out[k] = struct{}{}
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
