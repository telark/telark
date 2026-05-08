package policies

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
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

func PolicyName(planID, templateID string, scope ScopeSpec) string {
	return fmt.Sprintf("plsyro-%s-%s-%s", planID, templateID, scopeSuffix(scope))
}

func scopeSuffix(scope ScopeSpec) string {
	apps := append([]string(nil), scope.ApplicationIDs...)
	sort.Strings(apps)
	h := sha256.Sum256([]byte(scope.Namespace + "\x00" + joinSorted(apps)))
	return hex.EncodeToString(h[:])[:scopeHashLength]
}

func joinSorted(items []string) string {
	var out strings.Builder
	for i, s := range items {
		if i > 0 {
			out.WriteString(",")
		}
		out.WriteString(s)
	}
	return out.String()
}

func PolicyMeta(meta RenderMeta, templateID string, scope ScopeSpec) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      PolicyName(meta.PlanID, templateID, scope),
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
	if len(appIDs) == 0 {
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

func PolicyShell(meta RenderMeta, templateID string, scope ScopeSpec) *kyvernov1.Policy {
	bg := false
	return &kyvernov1.Policy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kyverno.io/v1",
			Kind:       "Policy",
		},
		ObjectMeta: PolicyMeta(meta, templateID, scope),
		Spec: kyvernov1.Spec{
			ValidationFailureAction: FailureAction(meta.Mode),
			Background:              &bg,
		},
	}
}
