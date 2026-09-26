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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

func (a *Applier) listByPlan(ctx context.Context, planID string) ([]unstructured.Unstructured, error) {
	selector := fmt.Sprintf(labelSelectorFormat, datapolicies.LabelPlanID, planID)
	list, err := a.dyn.Resource(KyvernoPolicyGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (a *Applier) CleanupByPlanID(ctx context.Context, planID string) error {
	items, err := a.listByPlan(ctx, planID)
	if err != nil {
		return err
	}
	return a.deletePolicies(ctx, items, nil)
}

func (a *Applier) DeletePoliciesByLabelAndNames(ctx context.Context, planID string, names []string) error {
	if len(names) == constants.DefaultInitValue {
		return nil
	}
	wanted := make(map[string]struct{}, len(names))
	for _, n := range names {
		wanted[n] = struct{}{}
	}
	items, err := a.listByPlan(ctx, planID)
	if err != nil {
		return err
	}
	return a.deletePolicies(ctx, items, wanted)
}

// A nil wanted set deletes every item.
func (a *Applier) deletePolicies(ctx context.Context, items []unstructured.Unstructured, wanted map[string]struct{}) error {
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(constants.PolicyOpConcurrency)
	for i := range items {
		item := &items[i]
		if wanted != nil {
			if _, ok := wanted[item.GetName()]; !ok {
				continue
			}
		}
		ns, name := item.GetNamespace(), item.GetName()
		g.Go(func() error {
			return a.deletePolicy(gctx, ns, name)
		})
	}
	return g.Wait()
}

// Returns the first patch error so the caller can roll back.
func (a *Applier) PatchPoliciesMode(ctx context.Context, planID, newMode string) error {
	items, err := a.listByPlan(ctx, planID)
	if err != nil {
		return fmt.Errorf("list policies for plan %q: %w", planID, err)
	}
	if len(items) == constants.DefaultInitValue {
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
	for i := range items {
		item := &items[i]
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
			return a.deletePolicy(gctx, ns, name)
		})
	}
	return g.Wait()
}

// A policy someone else already removed is the state the caller wanted, so NotFound is a success.
func (a *Applier) deletePolicy(ctx context.Context, namespace, name string) error {
	err := a.dyn.Resource(KyvernoPolicyGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf(errDeletePolicyFmt, namespace, name, err)
	}
	return nil
}
