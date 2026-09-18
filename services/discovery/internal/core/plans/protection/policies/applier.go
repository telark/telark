package policies

import (
	"context"
	"encoding/json"
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	datapolicies "github.com/telark/data/policies"
	"github.com/telark/discovery/internal/constants"
	kcoreapply "github.com/telark/kcore/ops/apply"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

type Applier struct {
	dyn    dynamic.Interface
	mapper meta.RESTMapper
}

func NewApplier(dyn dynamic.Interface, mapper meta.RESTMapper) *Applier {
	return &Applier{dyn: dyn, mapper: mapper}
}

func (a *Applier) Deploy(ctx context.Context, rendered []kyvernov1.Policy) ([]string, error) {
	if len(rendered) == constants.DefaultInitValue {
		return nil, nil
	}

	resources := make([]unstructured.Unstructured, constants.DefaultInitValue, len(rendered))
	names := make([]string, constants.DefaultInitValue, len(rendered))
	for i := range rendered {
		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&rendered[i])
		if err != nil {
			return nil, fmt.Errorf("convert policy %q: %w", rendered[i].Name, err)
		}
		resources = append(resources, unstructured.Unstructured{Object: obj})
		names = append(names, rendered[i].Name)
	}

	err := kcoreapply.ApplyUnstructuredServerSide(
		ctx,
		a.dyn,
		a.mapper,
		resources,
		kcoreapply.ServerSideApplyOptions{
			FieldManager: FieldManager,
			Force:        true,
			DryRun:       false,
		},
		nil,
	)
	if err != nil {
		return nil, err
	}
	return names, nil
}

func (a *Applier) CleanupByPlanID(ctx context.Context, planID string) error {
	selector := fmt.Sprintf(labelSelectorFormat, datapolicies.LabelPlanID, planID)
	list, err := a.dyn.Resource(KyvernoPolicyGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return err
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(constants.PolicyOpConcurrency)
	for i := range list.Items {
		item := &list.Items[i]
		ns, name := item.GetNamespace(), item.GetName()
		g.Go(func() error {
			_ = a.dyn.Resource(KyvernoPolicyGVR).
				Namespace(ns).
				Delete(gctx, name, metav1.DeleteOptions{})
			return nil
		})
	}
	_ = g.Wait()
	return nil
}

func (a *Applier) DeletePoliciesByLabelAndNames(ctx context.Context, planID string, names []string) error {
	if len(names) == constants.DefaultInitValue {
		return nil
	}
	wanted := make(map[string]struct{}, len(names))
	for _, n := range names {
		wanted[n] = struct{}{}
	}
	selector := fmt.Sprintf(labelSelectorFormat, datapolicies.LabelPlanID, planID)
	list, err := a.dyn.Resource(KyvernoPolicyGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return err
	}
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(constants.PolicyOpConcurrency)
	for i := range list.Items {
		item := &list.Items[i]
		if _, ok := wanted[item.GetName()]; !ok {
			continue
		}
		ns, name := item.GetNamespace(), item.GetName()
		g.Go(func() error {
			_ = a.dyn.Resource(KyvernoPolicyGVR).
				Namespace(ns).
				Delete(gctx, name, metav1.DeleteOptions{})
			return nil
		})
	}
	_ = g.Wait()
	return nil
}

// Returns the first patch error so the caller can roll back.
func (a *Applier) PatchPoliciesMode(ctx context.Context, planID, newMode string) error {
	selector := fmt.Sprintf(labelSelectorFormat, datapolicies.LabelPlanID, planID)
	list, err := a.dyn.Resource(KyvernoPolicyGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return fmt.Errorf("list policies for plan %q: %w", planID, err)
	}
	if len(list.Items) == constants.DefaultInitValue {
		return nil
	}
	action := datapolicies.FailureAction(newMode)
	patch := map[string]any{
		"spec": map[string]any{
			"validationFailureAction": action,
		},
	}
	body, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshal mode patch: %w", err)
	}
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(constants.PolicyOpConcurrency)
	for i := range list.Items {
		item := &list.Items[i]
		ns, name := item.GetNamespace(), item.GetName()
		g.Go(func() error {
			if _, err := a.dyn.Resource(KyvernoPolicyGVR).
				Namespace(ns).
				Patch(gctx, name, types.MergePatchType, body, metav1.PatchOptions{
					FieldManager: FieldManager,
				}); err != nil {
				return fmt.Errorf("patch policy %s/%s mode: %w", ns, name, err)
			}
			return nil
		})
	}
	return g.Wait()
}

func (a *Applier) DeletePoliciesByNamespacedName(ctx context.Context, refs []NamespacedName) error {
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(constants.PolicyOpConcurrency)
	for _, ref := range refs {
		ns, name := ref.Namespace, ref.Name
		g.Go(func() error {
			_ = a.dyn.Resource(KyvernoPolicyGVR).
				Namespace(ns).
				Delete(gctx, name, metav1.DeleteOptions{})
			return nil
		})
	}
	_ = g.Wait()
	return nil
}
