package rollback

import (
	"context"
	"fmt"

	kcoremanifest "github.com/telark/telark/internal/kcore/manifest"
	"github.com/telark/telark/internal/kcore/resilience/retry"
	"github.com/telark/telark/services/discovery/internal/constants"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// Exported for tests (Run's in-cluster client is beyond SetDynamicClient). A server-side apply keeps every field another
// manager owns, so a field added after the snapshot survived a "successful" rollback; a replace puts it back as snapshotted.
func ReplaceUnstructured(
	ctx context.Context,
	dyn dynamic.Interface,
	mapper meta.RESTMapper,
	resources []unstructured.Unstructured,
	dryRun bool,
	onReplaced func(kind, name, namespace string),
) error {
	for i := range resources {
		res := *resources[i].DeepCopy()
		kcoremanifest.CleanManifestForApply(res.Object)
		gvk := res.GroupVersionKind()
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return err
		}
		var ri dynamic.ResourceInterface = dyn.Resource(mapping.Resource)
		if ns := res.GetNamespace(); ns != constants.EmptyString {
			ri = dyn.Resource(mapping.Resource).Namespace(ns)
		}
		err = retry.OnTransient(ctx, retry.DefaultApply(), func() error {
			return replaceOne(ctx, ri, &res, dryRun)
		})
		if err != nil {
			return fmt.Errorf(string(constants.ErrRollbackReplaceFailed), res.GetKind(), res.GetName(), err)
		}
		if onReplaced != nil {
			onReplaced(res.GetKind(), res.GetName(), res.GetNamespace())
		}
	}
	return nil
}

func replaceOne(ctx context.Context, ri dynamic.ResourceInterface, res *unstructured.Unstructured, dryRun bool) error {
	var dryRunOpt []string
	if dryRun {
		dryRunOpt = []string{metav1.DryRunAll}
	}
	live, err := ri.Get(ctx, res.GetName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = ri.Create(ctx, res, metav1.CreateOptions{FieldManager: constants.RollbackFieldManager, DryRun: dryRunOpt})
		return err
	}
	if err != nil {
		return err
	}
	// The live resourceVersion keeps the replace conditional: a write racing it conflicts and is retried.
	res.SetResourceVersion(live.GetResourceVersion())
	// The snapshot's owners are stripped as possibly stale; the live ones keep an operator's
	// object from being orphaned by the full replace.
	res.SetOwnerReferences(live.GetOwnerReferences())
	_, err = ri.Update(ctx, res, metav1.UpdateOptions{FieldManager: constants.RollbackFieldManager, DryRun: dryRunOpt})
	return err
}
