package listing

import (
	"context"
	"slices"

	"github.com/telark/discovery/constants"
	gcfghelper "github.com/telark/discovery/helpers/globalconfig"
	kcorecore "github.com/telark/kcore/resources/core"
	kcoregroup "github.com/telark/kcore/resources/group"
)

var InformersCache func(context.Context, []string) ([]kcoregroup.ResourceRef, bool)

// Resources lists application-relevant resources across the given namespaces.
// Excluded namespaces are filtered out at the boundary so both the informer-cache
// fast path and the live-list fallback honor the exclusion list. Empty input
// means "all namespaces" — expanded explicitly with exclusion applied so the
// fallback can never see excluded namespaces.
func Resources(ctx context.Context, namespaces []string) ([]kcoregroup.ResourceRef, error) {
	resolved, err := resolveNamespaces(ctx, namespaces)
	if err != nil {
		return nil, err
	}
	if len(resolved) == constants.DefaultInitValue {
		return nil, nil
	}
	if InformersCache != nil {
		if refs, ok := InformersCache(ctx, resolved); ok {
			return refs, nil
		}
	}
	return kcoregroup.ListAllResourcesInNamespaces(resolved)
}

func resolveNamespaces(ctx context.Context, namespaces []string) ([]string, error) {
	if len(namespaces) == constants.DefaultInitValue {
		return allNonExcludedNamespaces(ctx)
	}
	return filterExcludedNamespaces(ctx, namespaces), nil
}

func allNonExcludedNamespaces(ctx context.Context) ([]string, error) {
	nsList, err := kcorecore.GetAllNamespaces()
	if err != nil {
		return nil, err
	}
	excluded := gcfghelper.FetchExcludedNamespaces(ctx)
	out := make([]string, constants.DefaultInitValue, len(nsList))
	for i := range nsList {
		name := nsList[i].Name
		if name == constants.EmptyString {
			continue
		}
		if slices.Contains(excluded, name) {
			continue
		}
		out = append(out, name)
	}
	return out, nil
}

func filterExcludedNamespaces(ctx context.Context, namespaces []string) []string {
	excluded := gcfghelper.FetchExcludedNamespaces(ctx)
	if len(excluded) == constants.DefaultInitValue {
		return namespaces
	}
	out := make([]string, constants.DefaultInitValue, len(namespaces))
	for _, ns := range namespaces {
		if slices.Contains(excluded, ns) {
			continue
		}
		out = append(out, ns)
	}
	return out
}
