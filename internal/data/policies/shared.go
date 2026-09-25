package policies

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/telark/data/constants"
	"github.com/telark/data/plans"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	LabelPlanID    = "telark.erpi/protection-plan"
	LabelTemplate  = "telark.erpi/template-id"
	LabelManagedBy = "telark.erpi/managed-by"
	ManagedByValue = "telark"

	AnnotationPlanName  = "telark.erpi/plan-name"
	AnnotationCreatedBy = "telark.erpi/created-by"

	AppNameLabel = "app.kubernetes.io/name"

	scopeHashLength = 8
)

const (
	KindCronJob = "CronJob"

	// A CronJob nests its pod template one level deeper than every other workload kind, so a
	// rule written for spec.template.spec resolves to null on a CronJob and admits it.
	PodSpecPath        = "spec.template.spec"
	CronJobPodSpecPath = "spec.jobTemplate.spec.template.spec"

	cronJobRuleSuffix = "-cronjob"
)

// telark renders namespaced Policy objects, and a namespaced policy may not reference a
// cluster-scoped kind, so ClusterPolicy cannot appear here even though it is the other half of
// Kyverno's policy API.
var KyvernoPolicyKinds = []string{"Policy"}

// ControllerSubjects are the in-cluster identities that keep workloads running. They are matched
// on the authenticated request, not on the object being admitted — unlike a label on the
// incoming resource, which is data supplied by whoever is making the change. Both forms of
// kube-controller-manager identity are listed because it authenticates as its own user when
// --use-service-account-credentials is off and as a per-controller kube-system service account
// when it is on, and the replacement Pod arrives from whichever is in use.
var ControllerSubjects = []rbacv1.Subject{
	{Kind: rbacv1.UserKind, Name: "system:kube-controller-manager"},
	{Kind: rbacv1.UserKind, Name: "system:kube-scheduler"},
	{Kind: rbacv1.GroupKind, Name: "system:serviceaccounts:kube-system"},
}

// ControllerCreatedKinds are never created by a person, so a rule that can reach one must carry
// ExcludePlatformWrites or it denies the controller that keeps the workload alive.
var ControllerCreatedKinds = []string{
	"Pod",
	"ReplicaSet",
	"Job",
	"PersistentVolumeClaim",
	"ControllerRevision",
}

// AppIdentityKinds are the kinds an application is composed of that a person creates. Every
// kind a built-in controller creates on its own is absent on purpose: workloads propagate
// app.kubernetes.io/name onto what they create, so an identity match reaching Pod would deny
// the replicaset-controller's own CREATE and leave the application unable to replace a pod
// after an eviction, a node loss or an OOM-kill. Excluded for that reason: Pod, ReplicaSet,
// ControllerRevision, Job (created by a CronJob) and PersistentVolumeClaim (created from a
// StatefulSet's volumeClaimTemplates). Every entry is a built-in API so the rendered kind
// always resolves.
var AppIdentityKinds = []string{
	"ConfigMap",
	"CronJob",
	"DaemonSet",
	"Deployment",
	"HorizontalPodAutoscaler",
	"Ingress",
	"NetworkPolicy",
	"Secret",
	"Service",
	"ServiceAccount",
	"StatefulSet",
}

var (
	StandardWorkloadKinds = []string{"Deployment", "StatefulSet", "DaemonSet", "Job"}
	CronJobKinds          = []string{KindCronJob}
	WorkloadKinds         = append(slices.Clone(StandardWorkloadKinds), CronJobKinds...)

	podSpecVariants = []struct {
		kinds      []string
		path       string
		ruleSuffix string
	}{
		{StandardWorkloadKinds, PodSpecPath, constants.EmptyString},
		{CronJobKinds, CronJobPodSpecPath, cronJobRuleSuffix},
	}
)

func PolicyName(planID, templateCode string, scope ScopeSpec) string {
	return fmt.Sprintf("telark-%s-%s-%s", planID, templateCode, scopeSuffix(scope))
}

func scopeSuffix(scope ScopeSpec) string {
	apps := append([]string(nil), scope.ApplicationIDs...)
	slices.Sort(apps)
	h := sha256.Sum256([]byte(scope.Namespace + "\x00" + strings.Join(apps, ",")))
	return hex.EncodeToString(h[:])[:scopeHashLength]
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
	if mode == plans.ModeEnforce {
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

// ExcludePlatformWrites exempts telark's own rendered policies, so a plan can still deploy and
// repair its rules in a namespace another plan is freezing. Kind, not label: a Deployment
// cannot claim to be a Policy, and writing Kyverno policies is already a platform-level
// privilege that outranks any plan. Every rule carries this.
func ExcludePlatformWrites() *kyvernov1.MatchResources {
	return &kyvernov1.MatchResources{
		Any: kyvernov1.ResourceFilters{
			{ResourceDescription: kyvernov1.ResourceDescription{Kinds: append([]string(nil), KyvernoPolicyKinds...)}},
		},
	}
}

// ExcludeControllerWrites additionally exempts the controller identities, and belongs only on
// rules that reach objects a controller creates or deletes on its own — a replacement Pod, a
// ReplicaSet during a rollout, a PVC being bound.
//
// It must never go on a rule that matches user-authored workload spec or the scale
// subresource. The HPA controller shares these identities, so a blanket exclusion let
// `kubectl autoscale` scale a workload straight through a replica freeze.
//
// The cut has to be per rule rather than per identity: with
// --use-service-account-credentials=false the kube-controller-manager runs every controller
// under the single `system:kube-controller-manager` user, so the HPA and the replicaset
// controller are literally indistinguishable by subject. Only the rule a write lands on can
// separate them.
func ExcludeControllerWrites() *kyvernov1.MatchResources {
	exclude := ExcludePlatformWrites()
	exclude.Any = append(exclude.Any, kyvernov1.ResourceFilter{
		UserInfo: kyvernov1.UserInfo{Subjects: append([]rbacv1.Subject(nil), ControllerSubjects...)},
	})
	return exclude
}

// BuildIdentityMatch adds a filter on the application's identity label to the name-based match.
// Every name an application owns already exists, so a name-only CREATE rule can block
// re-creation of a deleted member but never a new resource joining the application. The label
// selector is only added when the application set is non-empty, so namespace scope is unchanged
// and a workload set carrying no identity label falls back to the name-only reach rather than
// matching every resource in the namespace. The kinds are AppIdentityKinds, never the caller's,
// so the selector can only ever reach resources a person creates.
func BuildIdentityMatch(scope ScopeSpec, kinds, ops []string) (kyvernov1.MatchResources, bool) {
	match, ok := BuildMatch(scope, kinds, ops)
	selector := AppScopeSelector(scope.ApplicationIDs)
	if selector == nil {
		return match, ok
	}
	rd := kyvernov1.ResourceDescription{
		Kinds:      append([]string(nil), AppIdentityKinds...),
		Selector:   selector,
		Operations: admissionOps(ops),
	}
	if !ok {
		return kyvernov1.MatchResources{Any: kyvernov1.ResourceFilters{{ResourceDescription: rd}}}, true
	}
	match.Any = append(match.Any, kyvernov1.ResourceFilter{ResourceDescription: rd})
	return match, true
}

// MatchNamed matches an explicit name list rather than the application's owned resources, for
// resources an application references but does not own. Names may carry wildcards.
func MatchNamed(kinds, ops, names []string) (kyvernov1.MatchResources, bool) {
	if len(names) == constants.DefaultInitValue {
		return kyvernov1.MatchResources{}, false
	}
	rd := kyvernov1.ResourceDescription{
		Kinds:      append([]string(nil), kinds...),
		Names:      append([]string(nil), names...),
		Operations: admissionOps(ops),
	}
	return kyvernov1.MatchResources{Any: kyvernov1.ResourceFilters{{ResourceDescription: rd}}}, true
}

func admissionOps(ops []string) []kyvernov1.AdmissionOperation {
	if len(ops) == constants.DefaultInitValue {
		return nil
	}
	out := make([]kyvernov1.AdmissionOperation, constants.DefaultInitValue, len(ops))
	for _, op := range ops {
		out = append(out, kyvernov1.AdmissionOperation(op))
	}
	return out
}

func MatchAllAny(kinds []string, ops []string, appIDs []string) kyvernov1.MatchResources {
	rd := kyvernov1.ResourceDescription{
		Kinds:      append([]string(nil), kinds...),
		Selector:   AppScopeSelector(appIDs),
		Operations: admissionOps(ops),
	}
	return kyvernov1.MatchResources{
		Any: kyvernov1.ResourceFilters{
			{ResourceDescription: rd},
		},
	}
}

const (
	KindWildcard       = "*"
	scaleSuffix        = "/scale"
	subresourceSepRune = '/'
)

// Kyverno's `*` kind selector does not match subresources, so `kubectl scale` walks straight
// past a rule that only names the kind. These are enumerated rather than matched as `*/scale`
// because Kyverno rejects a match entry that mixes `*` with any other kind, and `*/*` would
// also cover `status`, which kubelet and the built-in controllers write — blocking that is an
// outage, not a freeze.
var ScaleSubresourceKinds = []string{
	"Deployment" + scaleSuffix,
	"StatefulSet" + scaleSuffix,
	"ReplicaSet" + scaleSuffix,
	"ReplicationController" + scaleSuffix,
}

func baseKind(kind string) string {
	idx := strings.IndexRune(kind, subresourceSepRune)
	if idx < constants.DefaultInitValue {
		return kind
	}
	return kind[:idx]
}

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
		names := grouped[baseKind(k)]
		if len(names) == constants.DefaultInitValue {
			continue
		}
		// Namespaced Policy: do NOT set Namespaces. Kyverno admission rejects
		// `match.any[].resources.namespaces[]` on namespaced Policy. Scope comes
		// from ObjectMeta.Namespace on the Policy itself.
		rd := kyvernov1.ResourceDescription{
			Kinds:      []string{k},
			Names:      append([]string(nil), names...),
			Operations: admissionOps(ops),
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
		if _, ok := grouped[baseKind(k)]; ok {
			out = append(out, k)
		}
	}
	return out
}

type SingleRuleSpec struct {
	TemplateID   string
	TemplateCode string
	RuleName     string
	Kinds        []string
	Ops          []string
	Message      string
	Deny         *kyvernov1.Deny
	// MatchAppIdentity also matches resources carrying the application's identity label, for
	// rules that must reach resources the application does not own yet.
	MatchAppIdentity bool
	// ExcludeControllers exempts the controller identities. Set it only when the rule reaches
	// objects a controller creates or deletes on its own; see ExcludeControllerWrites.
	ExcludeControllers bool
}

func excludeFor(controllers bool) *kyvernov1.MatchResources {
	if controllers {
		return ExcludeControllerWrites()
	}
	return ExcludePlatformWrites()
}

func RenderSingleRulePolicy(meta RenderMeta, scope ScopeSpec, spec SingleRuleSpec) *kyvernov1.Policy {
	build := BuildMatch
	if spec.MatchAppIdentity {
		build = BuildIdentityMatch
	}
	match, ok := build(scope, spec.Kinds, spec.Ops)
	if !ok {
		return nil
	}
	pol := PolicyShell(meta, spec.TemplateID, spec.TemplateCode, scope)
	pol.Spec.Rules = []kyvernov1.Rule{
		{
			Name:             spec.RuleName,
			MatchResources:   match,
			ExcludeResources: excludeFor(spec.ExcludeControllers),
			Validation:       Validation(spec.Message, spec.Deny),
		},
	}
	return pol
}

// Kyverno defaults allowExistingViolations to true, which exempts a workload that already
// violates the rule from every later update — during a freeze that lets an operator swap one
// blocked image for another. A protection plan must freeze its worst offenders too.
func Validation(message string, deny *kyvernov1.Deny) *kyvernov1.Validation {
	allowExistingViolations := false
	return &kyvernov1.Validation{
		Message:                 message,
		Deny:                    deny,
		AllowExistingViolations: &allowExistingViolations,
	}
}

// PodSpecRuleSpec describes a rule whose deny expression reads a pod spec. Deny takes the
// pod-spec path because the rule is emitted once per pod-template shape.
type PodSpecRuleSpec struct {
	TemplateID   string
	TemplateCode string
	RuleName     string
	Ops          []string
	Message      string
	Deny         func(podSpecPath string) *kyvernov1.Deny
}

// PodSpecRules emits one rule per pod-template shape so CronJob is evaluated at its own path
// instead of being silently admitted by a rule that only knows spec.template.spec.
func PodSpecRules(scope ScopeSpec, spec PodSpecRuleSpec) []kyvernov1.Rule {
	rules := make([]kyvernov1.Rule, constants.DefaultInitValue, len(podSpecVariants))
	for _, variant := range podSpecVariants {
		match, ok := BuildMatch(scope, variant.kinds, spec.Ops)
		if !ok {
			continue
		}
		rules = append(rules, kyvernov1.Rule{
			Name:             spec.RuleName + variant.ruleSuffix,
			MatchResources:   match,
			ExcludeResources: ExcludePlatformWrites(),
			Validation:       Validation(spec.Message, spec.Deny(variant.path)),
		})
	}
	return rules
}

func RenderPodSpecRulePolicy(meta RenderMeta, scope ScopeSpec, spec PodSpecRuleSpec) *kyvernov1.Policy {
	rules := PodSpecRules(scope, spec)
	if len(rules) == constants.DefaultInitValue {
		return nil
	}
	pol := PolicyShell(meta, spec.TemplateID, spec.TemplateCode, scope)
	pol.Spec.Rules = rules
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
