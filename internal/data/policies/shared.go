package policies

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	"github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/data/plans"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	LabelPlanID    = "telark.io/protection-plan"
	LabelTemplate  = "telark.io/template-id"
	LabelManagedBy = "app.kubernetes.io/managed-by"
	ManagedByValue = "telark"

	AnnotationPlanName  = "telark.io/plan-name"
	AnnotationCreatedBy = "telark.io/created-by"
	// Health compares it with a fresh render, so a policy left behind by an older renderer is
	// redeployed instead of trusted. Absent on policies rendered before it existed.
	AnnotationRenderHash = "telark.io/render-hash"

	AppNameLabel = "app.kubernetes.io/name"

	PolicyKind = policieskyvernoio.NamespacedValidatingPolicyKind

	scopeHashLength = 8

	ConditionScope       = "scope"
	ConditionNotPlatform = "not-platform-write"
	ConditionNotExcluded = "not-excluded"
)

const KindCronJob = "CronJob"

// The legacy kind stays for the 0.1.x policies an upgrade leaves behind: deleting them must not be refused.
var KyvernoPolicyKinds = []string{"Policy", PolicyKind}

// Kyverno's reports controller writes these into the target namespace itself; a wildcard rule
// would deny them in enforce and flood the violations in audit.
var KyvernoReportKinds = []string{"PolicyReport", "EphemeralReport"}

var PlatformKinds = slices.Concat(KyvernoPolicyKinds, KyvernoReportKinds)

// Matched on the authenticated identity, which a writer cannot forge the way a resource label can.
// kube-controller-manager is its own user with --use-service-account-credentials off and a
// per-controller kube-system service account with it on, so both forms are listed.
var ControllerSubjects = []rbacv1.Subject{
	{Kind: rbacv1.UserKind, Name: "system:kube-controller-manager"},
	{Kind: rbacv1.UserKind, Name: "system:kube-scheduler"},
	{Kind: rbacv1.GroupKind, Name: "system:serviceaccounts:kube-system"},
}

// ControllerCreatedKinds are never created by a person, so a rule that can reach one must carry
// ExceptControllers or it denies the controller that keeps the workload alive.
var ControllerCreatedKinds = []string{
	"Pod",
	"ReplicaSet",
	"Job",
	"PersistentVolumeClaim",
	"ControllerRevision",
}

// Only kinds a person creates, all built-in APIs so the rendered kind always resolves. Workloads
// propagate app.kubernetes.io/name onto what they create, so reaching a ControllerCreatedKind
// would deny the replicaset-controller's own CREATE after an eviction, node loss or OOM-kill.
var AppIdentityKinds = []string{
	"ConfigMap",
	KindCronJob,
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

var WorkloadKinds = []string{"Deployment", "StatefulSet", "DaemonSet", "Job", KindCronJob}

func PolicyName(planID, templateCode string, scope ScopeSpec) string {
	return fmt.Sprintf("telark-%s-%s-%s", planID, templateCode, scopeSuffix(scope))
}

func scopeSuffix(scope ScopeSpec) string {
	apps := append([]string(nil), scope.ApplicationRefs...)
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

func RenderHash(pol *policiesv1.NamespacedValidatingPolicy) string {
	raw, err := json.Marshal(pol.Spec)
	if err != nil {
		return constants.EmptyString
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func ValidationActions(mode string) []admissionregistrationv1.ValidationAction {
	if mode == plans.ModeEnforce {
		return []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny}
	}
	return []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit}
}

// A name-only CREATE rule blocks re-creating a deleted member but never a new resource joining
// the application, so the identity label is matched too. The label values are the app set and
// the kinds are AppIdentityKinds, never the caller's, so it only reaches person-created kinds.
func BuildIdentityMatch(scope ScopeSpec, kinds, ops []string) ([]ResourceFilter, bool) {
	match, ok := BuildMatch(scope, kinds, ops)
	if len(scope.ApplicationRefs) == constants.DefaultInitValue {
		return match, ok
	}
	identity := ResourceFilter{
		Kinds:  slices.Clone(AppIdentityKinds),
		AppIDs: slices.Clone(scope.ApplicationRefs),
		Ops:    ops,
	}
	return append(match, identity), true
}

// MatchNamed matches an explicit name list rather than the application's owned resources, for
// resources an application references but does not own. Names may carry wildcards.
func MatchNamed(kinds, ops, names []string) ([]ResourceFilter, bool) {
	if len(names) == constants.DefaultInitValue {
		return nil, false
	}
	return []ResourceFilter{{Kinds: slices.Clone(kinds), Names: slices.Clone(names), Ops: ops}}, true
}

const (
	KindWildcard = "*"
	scaleSuffix  = "/scale"
)

// A `*` kind never matches a subresource, so `kubectl scale` walks past it. Enumerated rather
// than `*/scale` or `*/*` (would deny `status`, which kubelet and the controllers write: an
// outage, not a freeze).
var ScaleSubresourceKinds = []string{
	"Deployment" + scaleSuffix,
	"StatefulSet" + scaleSuffix,
	kindReplicaSet + scaleSuffix,
	kindReplicationController + scaleSuffix,
}

func baseKind(kind string) string {
	base, _, _ := strings.Cut(kind, plans.SubresourceSeparator)
	return base
}

func BuildMatch(scope ScopeSpec, kinds []string, ops []string) ([]ResourceFilter, bool) {
	if len(scope.ApplicationRefs) == constants.DefaultInitValue {
		return []ResourceFilter{{Kinds: slices.Clone(kinds), Ops: ops}}, true
	}
	grouped := groupAppResourcesByKind(scope.AppResources, scope.Namespace)
	if len(grouped) == constants.DefaultInitValue {
		return nil, false
	}
	targetKinds := intersectKinds(kinds, grouped)
	filters := make([]ResourceFilter, constants.DefaultInitValue, len(targetKinds))
	for _, k := range targetKinds {
		names := grouped[baseKind(k)]
		if len(names) == constants.DefaultInitValue {
			continue
		}
		filters = append(filters, ResourceFilter{Kinds: []string{k}, Names: slices.Clone(names), Ops: ops})
	}
	return filters, len(filters) > constants.DefaultInitValue
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
		return slices.Sorted(maps.Keys(grouped))
	}
	out := make([]string, constants.DefaultInitValue, len(requested))
	for _, k := range requested {
		if _, ok := grouped[baseKind(k)]; ok {
			out = append(out, k)
		}
	}
	return out
}

// Only on branches that reach objects a controller writes itself (replacement Pod, PVC being bound), never on
// /scale or user-authored spec: the HPA controller shares these identities, and `kubectl autoscale` got through.
func ExceptControllers(filters []ResourceFilter) []ResourceFilter {
	for i := range filters {
		filters[i].ExceptControllers = true
	}
	return filters
}

type SingleRuleSpec struct {
	TemplateID   string
	TemplateCode string
	Kinds        []string
	Ops          []string
	Message      string
	// CEL that holds when a matched request is admitted.
	Admit string
	// MatchAppIdentity also matches resources carrying the application's identity label, for
	// rules that must reach resources the application does not own yet.
	MatchAppIdentity bool
	// ExcludeControllers exempts the controller identities. Set it only when the rule reaches
	// objects a controller creates or deletes on its own; see ExceptControllers.
	ExcludeControllers bool
}

func RenderSingleRulePolicy(meta RenderMeta, scope ScopeSpec, spec SingleRuleSpec) *policiesv1.NamespacedValidatingPolicy {
	build := BuildMatch
	if spec.MatchAppIdentity {
		build = BuildIdentityMatch
	}
	match, ok := build(scope, spec.Kinds, spec.Ops)
	if !ok {
		return nil
	}
	if spec.ExcludeControllers {
		match = ExceptControllers(match)
	}
	return NewPolicy(meta, scope, PolicySpec{
		TemplateID:   spec.TemplateID,
		TemplateCode: spec.TemplateCode,
		Match:        match,
		Validations:  []admissionregistrationv1.Validation{Validation(spec.Admit, spec.Message)},
	})
}

// The requester goes into the Event and the denial text through the message expression.
func Validation(admit, message string) admissionregistrationv1.Validation {
	return admissionregistrationv1.Validation{
		Expression:        admit,
		Message:           message,
		MessageExpression: CELString(message) + exprRequestedBySuffix,
	}
}

// Admit reads the pod spec through PodSpecVariables, which know a CronJob's deeper path.
type PodSpecRuleSpec struct {
	TemplateID   string
	TemplateCode string
	Ops          []string
	Message      string
	Admit        string
}

func RenderPodSpecRulePolicy(meta RenderMeta, scope ScopeSpec, spec PodSpecRuleSpec) *policiesv1.NamespacedValidatingPolicy {
	match, ok := BuildMatch(scope, WorkloadKinds, spec.Ops)
	if !ok {
		return nil
	}
	return NewPolicy(meta, scope, PolicySpec{
		TemplateID:   spec.TemplateID,
		TemplateCode: spec.TemplateCode,
		Match:        match,
		Variables:    PodSpecVariables(),
		Validations:  []admissionregistrationv1.Validation{Validation(spec.Admit, spec.Message)},
	})
}

func NewPolicy(meta RenderMeta, scope ScopeSpec, spec PolicySpec) *policiesv1.NamespacedValidatingPolicy {
	admission, background := true, false
	return &policiesv1.NamespacedValidatingPolicy{
		TypeMeta:   metav1.TypeMeta{APIVersion: policiesv1.GroupVersion.String(), Kind: PolicyKind},
		ObjectMeta: PolicyMeta(meta, spec.TemplateID, spec.TemplateCode, scope),
		Spec: policiesv1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{ResourceRules: resourceRules(spec.Match)},
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{Name: ConditionScope, Expression: AnyOf(spec.Match)},
				// By kind, which a Deployment can't fake: a plan must still repair its policies where another plan freezes.
				{Name: ConditionNotPlatform, Expression: celNegate(platformExpr)},
			},
			Variables:        spec.Variables,
			Validations:      spec.Validations,
			ValidationAction: ValidationActions(meta.Mode),
			// The CRD's only two defaults, set here so the live spec reads back exactly as rendered.
			EvaluationConfiguration: &policiesv1.EvaluationConfiguration{
				Admission:  &policiesv1.AdmissionConfiguration{Enabled: &admission},
				Background: &policiesv1.BackgroundConfiguration{Enabled: &background},
			},
		},
	}
}
