package apply

import (
	"context"
	"fmt"

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
		res := resources[i]
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

		var applyErr error
		if ns := res.GetNamespace(); ns != "" {
			_, applyErr = ri.Namespace(ns).Apply(ctx, res.GetName(), &res, applyOpts)
		} else {
			_, applyErr = ri.Apply(ctx, res.GetName(), &res, applyOpts)
		}
		if applyErr != nil {
			return fmt.Errorf("apply %s/%s: %w", res.GetKind(), res.GetName(), applyErr)
		}
		if onApplied != nil {
			onApplied(res.GetKind(), res.GetName(), res.GetNamespace())
		}
	}
	return nil
}
