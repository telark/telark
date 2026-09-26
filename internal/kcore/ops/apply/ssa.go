package apply

import (
	"context"
	"fmt"

	"github.com/telark/kcore/constants"
	"github.com/telark/kcore/manifest"
	"github.com/telark/kcore/resilience/retry"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

type ServerSideApplyOptions struct {
	FieldManager string
	Force        bool
	DryRun       bool
}

type ApplyLogFunc func(kind, name, namespace string)

func ApplyUnstructuredServerSide(
	ctx context.Context,
	dyn dynamic.Interface,
	mapper meta.RESTMapper,
	resources []unstructured.Unstructured,
	opts ServerSideApplyOptions,
	onApplied ApplyLogFunc,
) error {
	for i := range resources {
		// Apply a sanitized copy: an apply config must never carry
		// cluster-managed fields, and Force only overrides field-manager
		// conflicts, not the resourceVersion precondition.
		res := *resources[i].DeepCopy()
		manifest.CleanManifestForApply(res.Object)
		gvk := res.GroupVersionKind()

		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return err
		}

		gvr := mapping.Resource
		ri := dyn.Resource(gvr)
		applyOpts := metav1.ApplyOptions{FieldManager: opts.FieldManager, Force: opts.Force}
		if opts.DryRun {
			applyOpts.DryRun = []string{metav1.DryRunAll}
		}

		applyErr := retry.OnTransient(ctx, retry.DefaultApply(), func() error {
			var doErr error
			if ns := res.GetNamespace(); ns != constants.EmptyString {
				_, doErr = ri.Namespace(ns).Apply(ctx, res.GetName(), &res, applyOpts)
			} else {
				_, doErr = ri.Apply(ctx, res.GetName(), &res, applyOpts)
			}
			return doErr
		})
		if applyErr != nil {
			return fmt.Errorf("apply %s/%s: %w", res.GetKind(), res.GetName(), applyErr)
		}
		if onApplied != nil {
			onApplied(res.GetKind(), res.GetName(), res.GetNamespace())
		}
	}
	return nil
}
