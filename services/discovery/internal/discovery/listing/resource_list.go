package listing

import (
	"context"
	"slices"

	"github.com/telark/discovery/internal/constants"
	tcfghelper "github.com/telark/discovery/internal/helpers/telarkconfig"
	kcorecore "github.com/telark/kcore/resources/core"
	kcoregroup "github.com/telark/kcore/resources/group"
)

var InformersCache func(context.Context, []string) ([]kcoregroup.ResourceRef, bool)

var AppNamespacesCache func(ctx context.Context, appName string) []string

// A path listing only some of an app's namespaces publishes it without the
// rest, and every later per-app path lists only what the stored app still holds.
func AppNamespaces(ctx context.Context, appName string, known []string) []string {
	out := slices.Clone(known)
	if AppNamespacesCache == nil {
		return out
	}
	for _, ns := range AppNamespacesCache(ctx, appName) {
		if !slices.Contains(out, ns) {
			out = append(out, ns)
		}
	}
	return out
}

// Exclusions are applied at this boundary so both the informer-cache fast path and the
// live-list fallback honor them; empty input means "all namespaces", expanded explicitly.
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
	return filterExcludedNamespaces(ctx, namespaces)
}

func allNonExcludedNamespaces(ctx context.Context) ([]string, error) {
	nsList, err := kcorecore.GetAllNamespaces()
	if err != nil {
		return nil, err
	}
	excluded, err := tcfghelper.ExcludedNamespaces(ctx)
	if err != nil {
		return nil, err
	}
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

func filterExcludedNamespaces(ctx context.Context, namespaces []string) ([]string, error) {
	excluded, err := tcfghelper.ExcludedNamespaces(ctx)
	if err != nil {
		return nil, err
	}
	if len(excluded) == constants.DefaultInitValue {
		return namespaces, nil
	}
	out := make([]string, constants.DefaultInitValue, len(namespaces))
	for _, ns := range namespaces {
		if slices.Contains(excluded, ns) {
			continue
		}
		out = append(out, ns)
	}
	return out, nil
}
