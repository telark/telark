package policies

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/plsyro/data/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	LabelPlanID    = "plsyro.erpi/protection-plan"
	LabelTemplate  = "plsyro.erpi/template-id"
	LabelManagedBy = "plsyro.erpi/managed-by"
	ManagedByValue = "plsyro"

	AnnotationPlanName  = "plsyro.erpi/plan-name"
	AnnotationCreatedBy = "plsyro.erpi/created-by"

	AppNameLabel = "app.kubernetes.io/name"

	scopeHashLength = 8
)

var WorkloadKinds = []string{"Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob"}

func PolicyName(planID, templateCode string, scope ScopeSpec) string {
	return fmt.Sprintf("plsyro-%s-%s-%s", planID, templateCode, scopeSuffix(scope))
}

func scopeSuffix(scope ScopeSpec) string {
	apps := append([]string(nil), scope.ApplicationIDs...)
	slices.Sort(apps)
	h := sha256.Sum256([]byte(scope.Namespace + "\x00" + joinSorted(apps)))
	return hex.EncodeToString(h[:])[:scopeHashLength]
}

func joinSorted(items []string) string {
	var out strings.Builder
	for i, s := range items {
		if i > constants.DefaultInitValue {
			out.WriteString(",")
		}
		out.WriteString(s)
	}
	return out.String()
}

func PolicyMeta(meta RenderMeta, templateID, templateCode string, scope ScopeSpec) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      PolicyName(meta.PlanID, templateCode, scope),
		Namespace: scope.Namespace,
		Labels: map[string]string{
			LabelPlanID:    meta.PlanID,
			LabelTemplate:  templateID,
			LabelManagedBy: ManagedByValue,
		},
		Annotations: map[string]string{
			AnnotationPlanName:  meta.PlanName,
			AnnotationCreatedBy: meta.CreatedBy,
		},
	}
}

func FailureAction(mode string) kyvernov1.ValidationFailureAction {
	if mode == "enforce" {
		return kyvernov1.Enforce
	}
	return kyvernov1.Audit
}

func AppScopeSelector(appIDs []string) *metav1.LabelSelector {
	if len(appIDs) == constants.DefaultInitValue {
		return nil
	}
	return &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      AppNameLabel,
				Operator: metav1.LabelSelectorOpIn,
				Values:   append([]string(nil), appIDs...),
			},
		},
	}
}

func ExcludePlsyroManaged() *kyvernov1.MatchResources {
	return &kyvernov1.MatchResources{
		Any: kyvernov1.ResourceFilters{
			{
				ResourceDescription: kyvernov1.ResourceDescription{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							LabelManagedBy: ManagedByValue,
						},
					},
				},
			},
		},
	}
}

func MatchAllAny(kinds []string, ops []string, appIDs []string) kyvernov1.MatchResources {
	rd := kyvernov1.ResourceDescription{
		Kinds:    append([]string(nil), kinds...),
		Selector: AppScopeSelector(appIDs),
	}
	for _, op := range ops {
		rd.Operations = append(rd.Operations, kyvernov1.AdmissionOperation(op))
	}
	return kyvernov1.MatchResources{
		Any: kyvernov1.ResourceFilters{
			{ResourceDescription: rd},
		},
	}
}

const KindWildcard = "*"

// BuildMatch returns the Kyverno match block for a rule. For namespace scope it matches
// kinds + ops cluster-wide within the policy's namespace (unchanged behavior). For application
// scope it builds per-(kind, namespace) ResourceFilter entries with explicit Names taken from
// scope.AppResources. The boolean return is false when application scope has no resources for
// any of the requested kinds, signaling the caller to skip rendering this rule.
func BuildMatch(scope ScopeSpec, kinds []string, ops []string) (kyvernov1.MatchResources, bool) {
	if len(scope.ApplicationIDs) == constants.DefaultInitValue {
		return MatchAllAny(kinds, ops, nil), true
	}
	grouped := groupAppResourcesByKind(scope.AppResources, scope.Namespace)
	if len(grouped) == constants.DefaultInitValue {
		return kyvernov1.MatchResources{}, false
	}
	targetKinds := intersectKinds(kinds, grouped)
	if len(targetKinds) == constants.DefaultInitValue {
		return kyvernov1.MatchResources{}, false
	}
	filters := make(kyvernov1.ResourceFilters, constants.DefaultInitValue, len(targetKinds))
	for _, k := range targetKinds {
		names := grouped[k]
		if len(names) == constants.DefaultInitValue {
			continue
		}
		rd := kyvernov1.ResourceDescription{
			Kinds:      []string{k},
			Names:      append([]string(nil), names...),
			Namespaces: []string{scope.Namespace},
		}
		for _, op := range ops {
			rd.Operations = append(rd.Operations, kyvernov1.AdmissionOperation(op))
		}
		filters = append(filters, kyvernov1.ResourceFilter{ResourceDescription: rd})
	}
	if len(filters) == constants.DefaultInitValue {
		return kyvernov1.MatchResources{}, false
	}
	return kyvernov1.MatchResources{Any: filters}, true
}

func groupAppResourcesByKind(refs []ApplicationResourceRef, namespace string) map[string][]string {
	out := map[string][]string{}
	for _, ref := range refs {
		if ref.Namespace != namespace {
			continue
		}
		out[ref.Kind] = append(out[ref.Kind], ref.Name)
	}
	for k := range out {
		slices.Sort(out[k])
	}
	return out
}

func intersectKinds(requested []string, grouped map[string][]string) []string {
	if len(requested) == constants.SingleItem && requested[constants.DefaultInitValue] == KindWildcard {
		out := make([]string, constants.DefaultInitValue, len(grouped))
		for k := range grouped {
			out = append(out, k)
		}
		slices.Sort(out)
		return out
	}
	out := make([]string, constants.DefaultInitValue, len(requested))
	for _, k := range requested {
		if _, ok := grouped[k]; ok {
			out = append(out, k)
		}
	}
	return out
}

// SingleRuleSpec describes the per-template inputs to RenderSingleRulePolicy.
type SingleRuleSpec struct {
	TemplateID   string
	TemplateCode string
	RuleName     string
	Kinds        []string
	Ops          []string
	Message      string
	Deny         *kyvernov1.Deny
}

// RenderSingleRulePolicy is the canonical builder for templates that emit one Kyverno rule. It
// owns the BuildMatch + PolicyShell + ExcludePlsyroManaged wiring so each template only declares
// its own kinds/ops/message/deny.
func RenderSingleRulePolicy(meta RenderMeta, scope ScopeSpec, spec SingleRuleSpec) *kyvernov1.Policy {
	match, ok := BuildMatch(scope, spec.Kinds, spec.Ops)
	if !ok {
		return nil
	}
	pol := PolicyShell(meta, spec.TemplateID, spec.TemplateCode, scope)
	pol.Spec.Rules = []kyvernov1.Rule{
		{
			Name:             spec.RuleName,
			MatchResources:   match,
			ExcludeResources: ExcludePlsyroManaged(),
			Validation: &kyvernov1.Validation{
				Message: spec.Message,
				Deny:    spec.Deny,
			},
		},
	}
	return pol
}

func DenyWithConditions(allConditions []kyvernov1.Condition) *kyvernov1.Deny {
	return &kyvernov1.Deny{
		RawAnyAllConditions: &kyvernov1.ConditionsWrapper{
			Conditions: kyvernov1.AnyAllConditions{
				AllConditions: allConditions,
			},
		},
	}
}

func MakeCondition(key string, op kyvernov1.ConditionOperator, value any) kyvernov1.Condition {
	c := kyvernov1.Condition{Operator: op}
	c.SetKey(key)
	c.SetValue(value)
	return c
}

func PolicyShell(meta RenderMeta, templateID, templateCode string, scope ScopeSpec) *kyvernov1.Policy {
	bg := false
	return &kyvernov1.Policy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kyverno.io/v1",
			Kind:       "Policy",
		},
		ObjectMeta: PolicyMeta(meta, templateID, templateCode, scope),
		Spec: kyvernov1.Spec{
			ValidationFailureAction: FailureAction(meta.Mode),
			Background:              &bg,
		},
	}
}
