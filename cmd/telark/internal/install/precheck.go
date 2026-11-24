package install

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/telark/telark/cmd/telark/internal/check"
	"github.com/telark/telark/cmd/telark/internal/constants"
	"github.com/telark/telark/internal/data/metadata/base"
	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/data/plans"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/listtype"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/pruning"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"
)

// A rollback works one release back and the exporter refuses data two releases newer, so no release is skipped.
// Helm has already parsed every version here with the same semver package, so none fails to parse.
func step(installed, target string, published func() ([]string, error)) check.Result {
	from, to := semver.MustParse(installed), semver.MustParse(target)
	switch {
	case to.Equal(from):
		return check.Pass(constants.CheckStep, fmt.Sprintf(constants.MsgStepSame, target))
	case to.LessThan(from):
		return check.Fail(constants.CheckStep, fmt.Sprintf(constants.MsgStepDowngrade, target, installed))
	}
	tags, err := published()
	if err != nil {
		return check.Warn(constants.CheckStep, fmt.Sprintf(constants.MsgStepUnlisted, err))
	}
	skipped := slices.DeleteFunc(tags, func(tag string) bool {
		between := semver.MustParse(tag)
		return between.Prerelease() != constants.EmptyString || !between.GreaterThan(from) || !between.LessThan(to)
	})
	if len(skipped) > constants.DefaultInitValue {
		slices.Reverse(skipped)
		return check.Fail(constants.CheckStep, fmt.Sprintf(constants.MsgStepSkips, installed, target, strings.Join(skipped, constants.ListSeparator)))
	}
	return check.Pass(constants.CheckStep, fmt.Sprintf(constants.MsgStepNext, installed, target))
}

// Every stored object is checked the way the API server will check it under the target's CRDs: the schema, its CEL
// rules, its list types, and the fields the schema would prune, which is data the upgrade drops.
func storedObjects(ctx context.Context, core kubernetes.Interface, namespace, version, manifest string) check.Result {
	crds, err := telarkCRDs(manifest)
	if err != nil {
		return check.ReadFailed(constants.CheckStoredObjects, constants.ReadStoredObjects, err)
	}
	if len(crds) == constants.DefaultInitValue {
		return check.Warn(constants.CheckStoredObjects, constants.MsgObjectsNoCRDs)
	}
	objects := &objectCheck{core: core, namespace: namespace, version: version}
	for i := range crds {
		if err := objects.validate(ctx, &crds[i]); err != nil {
			return check.ReadFailed(constants.CheckStoredObjects, constants.ReadStoredObjects, err)
		}
	}
	return objects.result()
}

func telarkCRDs(manifest string) ([]apiextensionsv1.CustomResourceDefinition, error) {
	var crds []apiextensionsv1.CustomResourceDefinition
	for _, document := range releaseutil.SplitManifests(manifest) {
		// Another kind can type a field differently than a CRD does, so only CRDs are read as one.
		var head manifestHead
		if yaml.Unmarshal([]byte(document), &head) != nil || head.Kind != constants.CRDKind {
			continue
		}
		var crd apiextensionsv1.CustomResourceDefinition
		if err := yaml.Unmarshal([]byte(document), &crd); err != nil {
			return nil, err
		}
		if crd.Spec.Group == base.Group {
			crds = append(crds, crd)
		}
	}
	slices.SortFunc(crds, func(a, b apiextensionsv1.CustomResourceDefinition) int { return strings.Compare(a.Name, b.Name) })
	return crds, nil
}

func (o *objectCheck) validate(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition) error {
	version := crd.Spec.Versions[slices.IndexFunc(crd.Spec.Versions, func(v apiextensionsv1.CustomResourceDefinitionVersion) bool {
		return v.Storage
	})]
	var schema apiextensions.JSONSchemaProps
	if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(version.Schema.OpenAPIV3Schema, &schema, nil); err != nil {
		return err
	}
	validator, _, err := validation.NewSchemaValidator(&schema)
	if err != nil {
		return err
	}
	structural, err := structuralschema.NewStructural(&schema)
	if err != nil {
		return err
	}
	items, err := o.list(ctx, crd.Spec.Group, version.Name, crd.Spec.Names.Plural)
	if err != nil {
		return err
	}
	target := &crdSchema{plural: crd.Spec.Names.Plural, structural: structural, validator: validator,
		rules: cel.NewValidator(structural, true, celconfig.PerCallLimit)}
	for i := range items {
		o.check(ctx, target, &items[i])
	}
	return nil
}

// The API server prunes and defaults an object before it validates a write, so the check does the same.
func (o *objectCheck) check(ctx context.Context, target *crdSchema, item *unstructured.Unstructured) {
	o.total++
	object := item.Object
	unknown := structuralschema.UnknownFieldPathOptions{TrackUnknownFieldPaths: true}
	if dropped := pruning.PruneWithOptions(object, target.structural, true, unknown); len(dropped) > constants.DefaultInitValue {
		o.pruned = append(o.pruned, example(target.plural, item.GetName(), strings.Join(dropped, constants.ListSeparator)))
	}
	defaulting.Default(object, target.structural)
	problems := validation.ValidateCustomResource(nil, object, target.validator)
	problems = append(problems, listtype.ValidateListSetsAndMaps(nil, target.structural, object)...)
	celProblems, _ := target.rules.Validate(ctx, nil, target.structural, object, nil, celconfig.RuntimeCELCostBudget)
	if problems = append(problems, celProblems...); len(problems) > constants.DefaultInitValue {
		o.invalid = append(o.invalid, example(target.plural, item.GetName(), problems.ToAggregate().Error()))
	}
}

func example(plural, name, detail string) string {
	return fmt.Sprintf(constants.MsgObjectExample, plural, name, detail)
}

// A CRD the target adds holds no objects yet, so its missing list is no error.
func (o *objectCheck) list(ctx context.Context, group, version, plural string) ([]unstructured.Unstructured, error) {
	path := fmt.Sprintf(constants.TelarkResourcePath, group, version, o.namespace, plural)
	raw, err := o.core.Discovery().RESTClient().Get().AbsPath(path).DoRaw(ctx)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var list unstructured.UnstructuredList
	if err := list.UnmarshalJSON(raw); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (o *objectCheck) result() check.Result {
	if len(o.invalid) > constants.DefaultInitValue {
		return check.Fail(constants.CheckStoredObjects, fmt.Sprintf(constants.MsgObjectsInvalid, o.version, len(o.invalid), examples(o.invalid)))
	}
	if len(o.pruned) > constants.DefaultInitValue {
		return check.Warn(constants.CheckStoredObjects, fmt.Sprintf(constants.MsgObjectsPruned, o.version, len(o.pruned), examples(o.pruned)))
	}
	return check.Pass(constants.CheckStoredObjects, fmt.Sprintf(constants.MsgObjectsFit, o.total, o.version))
}

func examples(list []string) string {
	return strings.Join(list[:min(len(list), constants.MaxExamples)], constants.ListSeparator)
}

// The webhooks restart with Kyverno during an upgrade, which an enforcing plan should know about.
func enforcePlans(ctx context.Context, core kubernetes.Interface, namespace string) check.Result {
	path := fmt.Sprintf(constants.TelarkResourcePath, base.Group, base.V1Alpha1, namespace, v1alpha1.PluralProtectionPlans)
	raw, err := core.Discovery().RESTClient().Get().AbsPath(path).DoRaw(ctx)
	if err != nil {
		return check.ReadFailed(constants.CheckEnforcePlans, constants.ReadProtectionPlans, err)
	}
	var list planList
	if err := json.Unmarshal(raw, &list); err != nil {
		return check.ReadFailed(constants.CheckEnforcePlans, constants.ReadProtectionPlans, err)
	}
	enforcing := slices.DeleteFunc(list.Items, func(plan plan) bool {
		return plan.Spec.Mode != plans.ModeEnforce || plan.Status.Phase != plans.PhaseActive
	})
	if len(enforcing) > constants.DefaultInitValue {
		return check.Warn(constants.CheckEnforcePlans, fmt.Sprintf(constants.MsgEnforcePlans, len(enforcing)))
	}
	return check.Pass(constants.CheckEnforcePlans, constants.MsgNoEnforcePlans)
}

// Neither a recovery request nor an enforcing plan makes the upgrade unsafe, so failing to read one only warns.
func warnOnly(result check.Result) check.Result {
	if result.Status == constants.StatusFail {
		return check.Warn(result.Name, result.Message)
	}
	return result
}
