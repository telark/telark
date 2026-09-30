package informers

import (
	"slices"

	kcoreshared "github.com/telark/telark/internal/kcore/shared"
	"github.com/telark/telark/services/discovery/internal/constants"
)

// Every application kind, not only the kinds current apps use: the informer cache
// feeds discovery, so a kind that is not watched (a first CronJob) could never be discovered.
func watchKindSet() map[string]struct{} {
	out := make(map[string]struct{})
	for _, gvr := range kcoreshared.AppGVRs() {
		k := kcoreshared.ResourceKind(gvr.Resource)
		if k != constants.EmptyString && !informerExcludedKind(k) {
			out[k] = struct{}{}
		}
	}
	return out
}

func informerExcludedKind(kind string) bool {
	return slices.Contains(constants.InformerExcludedKinds, kind)
}
