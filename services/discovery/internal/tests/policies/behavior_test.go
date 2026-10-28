package policies

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	celutils "github.com/kyverno/sdk/extensions/cel/utils"
	"github.com/telark/telark/internal/data/plans"
	datapolicies "github.com/telark/telark/internal/data/policies"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/violations"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sadmission "k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/cel/lazy"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

const (
	statusPass        = "pass"
	statusFail        = "fail"
	statusSkip        = "skip"
	verdictDeny       = "deny"
	verdictAdmit      = "admit"
	human             = "test@example.com"
	controllerManager = "system:kube-controller-manager"
	kubeSystemSA      = "system:serviceaccount:kube-system:replicaset-controller"
	kubeSystemGroup   = "system:serviceaccounts:kube-system"
	otherName         = "other"
	claimName         = "data-wa1-0"
	keyName           = "name"
	keyNamespace      = "namespace"
	keyMetadata       = "metadata"
	keyVolumes        = "volumes"
	keyReplicas       = "replicas"
	keyTemplate       = "template"
	tplDelete         = "block-delete"
	tplUpdate         = "block-update"
	tplCreate         = "block-create"
	tplImageTypes     = "block-image-types"
	tplImageTags      = "block-image-tags"
	tplReplicas       = "block-replica-scaling"
	tplStorage        = "block-storage-changes"
	tplConfigSecret   = "block-config-secret-resource-changes"
	tplMounts         = "block-workload-config-mount-changes"
	replicasOne       = int64(1)
	replicasThree     = int64(3)
	digestImage       = "nginx@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

var (
	gvkDeployment  = schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	gvrDeployment  = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	gvrStatefulSet = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}
	gvkReplicaSet  = schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "ReplicaSet"}
	gvrReplicaSet  = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}
	gvkCronJob     = schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "CronJob"}
	gvrCronJob     = schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}
	gvkConfigMap   = schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
	gvrConfigMap   = schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	gvkSecret      = schema.GroupVersionKind{Version: "v1", Kind: "Secret"}
	gvrSecret      = schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
	gvkPod         = schema.GroupVersionKind{Version: "v1", Kind: "Pod"}
	gvrPod         = schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	gvkPVC         = schema.GroupVersionKind{Version: "v1", Kind: "PersistentVolumeClaim"}
	gvrPVC         = schema.GroupVersionResource{Version: "v1", Resource: "persistentvolumeclaims"}
	gvkScale       = schema.GroupVersionKind{Group: "autoscaling", Version: "v1", Kind: "Scale"}
	gvkPolicy      = schema.GroupVersionKind{Group: "policies.kyverno.io", Version: "v1", Kind: datapolicies.PolicyKind}
	gvrPolicy      = schema.GroupVersionResource{Group: "policies.kyverno.io", Version: "v1", Resource: "namespacedvalidatingpolicies"}
	gvkLegacy      = schema.GroupVersionKind{Group: "kyverno.io", Version: "v1", Kind: "Policy"}
	gvrLegacy      = schema.GroupVersionResource{Group: "kyverno.io", Version: "v1", Resource: "policies"}
	gvkReport      = schema.GroupVersionKind{Group: "wgpolicyk8s.io", Version: "v1alpha2", Kind: "PolicyReport"}
	gvrReport      = schema.GroupVersionResource{Group: "wgpolicyk8s.io", Version: "v1alpha2", Resource: "policyreports"}

	identity = map[string]any{datapolicies.AppNameLabel: acceptApp}
)

type admission struct {
	op        admissionv1.Operation
	gvk       schema.GroupVersionKind
	gvr       schema.GroupVersionResource
	sub       string
	name      string
	user      string
	groups    []string
	object    map[string]any
	oldObject map[string]any
}

type behaviorCase struct {
	name     string
	template string
	scope    string
	excluded bool
	req      admission
	want     string
}

type policyVerdict struct {
	status  string
	message string
	actions []admissionregistrationv1.ValidationAction
}

func object(gvk schema.GroupVersionKind, name string, labels map[string]any) map[string]any {
	meta := map[string]any{keyName: name, keyNamespace: acceptNamespace}
	if labels != nil {
		meta["labels"] = labels
	}
	return map[string]any{"apiVersion": gvk.GroupVersion().String(), "kind": gvk.Kind, keyMetadata: meta}
}

func container(name, image string) map[string]any {
	return map[string]any{keyName: name, "image": image}
}

func podSpec(containers ...map[string]any) map[string]any {
	list := make([]any, constants.DefaultInitValue, len(containers))
	for _, c := range containers {
		list = append(list, c)
	}
	return map[string]any{"containers": list}
}

func withSpec(obj, spec map[string]any) map[string]any {
	obj[keySpec] = spec
	return obj
}

func deployment(name string, replicas int64, spec map[string]any) map[string]any {
	return withSpec(object(gvkDeployment, name, nil), map[string]any{keyReplicas: replicas, keyTemplate: map[string]any{keySpec: spec}})
}

func create(gvk schema.GroupVersionKind, gvr schema.GroupVersionResource, obj map[string]any) admission {
	return admission{op: admissionv1.Create, gvk: gvk, gvr: gvr, name: objName(obj), user: human, object: obj}
}

func update(gvk schema.GroupVersionKind, gvr schema.GroupVersionResource, oldObj, obj map[string]any) admission {
	return admission{op: admissionv1.Update, gvk: gvk, gvr: gvr, name: objName(obj), user: human, object: obj, oldObject: oldObj}
}

func remove(gvk schema.GroupVersionKind, gvr schema.GroupVersionResource, name string) admission {
	return admission{op: admissionv1.Delete, gvk: gvk, gvr: gvr, name: name, user: human, oldObject: object(gvk, name, nil)}
}

func createNamed(gvk schema.GroupVersionKind, gvr schema.GroupVersionResource, labels map[string]any) admission {
	return create(gvk, gvr, object(gvk, otherName, labels))
}

// An update that only adds the identity label: a change, and nothing else.
func relabel(gvk schema.GroupVersionKind, gvr schema.GroupVersionResource, name string) admission {
	return update(gvk, gvr, object(gvk, name, nil), object(gvk, name, identity))
}

func createDeployment(spec map[string]any) admission {
	return create(gvkDeployment, gvrDeployment, deployment(otherName, replicasOne, spec))
}

func updateDeployment(oldSpec, spec map[string]any, replicas int64) admission {
	return update(gvkDeployment, gvrDeployment, deployment(otherName, replicasOne, oldSpec), deployment(otherName, replicas, spec))
}

func createCronJob(spec map[string]any) admission {
	jobTemplate := map[string]any{keySpec: map[string]any{keyTemplate: map[string]any{keySpec: spec}}}
	return create(gvkCronJob, gvrCronJob, withSpec(object(gvkCronJob, otherName, nil), map[string]any{"jobTemplate": jobTemplate}))
}

func scaleTo(gvr schema.GroupVersionResource, name string, replicas int64) admission {
	scale := func(n int64) map[string]any {
		return withSpec(object(gvkScale, name, nil), map[string]any{keyReplicas: n})
	}
	return admission{
		op: admissionv1.Update, gvk: gvkScale, gvr: gvr, sub: "scale", name: name, user: human,
		object: scale(replicas), oldObject: scale(replicasOne),
	}
}

func (a admission) by(username string, groups ...string) admission {
	a.user, a.groups = username, groups
	return a
}

func objName(obj map[string]any) string {
	return (&unstructured.Unstructured{Object: obj}).GetName()
}

func toJSON(t *testing.T, obj map[string]any) []byte {
	t.Helper()
	if obj == nil {
		return nil
	}
	raw, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

// Kyverno's matcher and compiled programs judge the request offline, in the order its vpol
// engine does (Policy.evaluateWithData): match conditions, lazy variables, then the first failing
// validation. The request is the admission request the webhook receives, converted the same way.
func handle(t *testing.T, templateID, scopeType, mode string, excluded bool, a admission) []policyVerdict {
	t.Helper()
	plan, resolved := acceptPlan(templateID, scopeType, mode, excluded)
	return handlePlan(t, plan, resolved, a)
}

func handlePlan(t *testing.T, plan *plans.ProtectionPlan, resolved map[string]datapolicies.ResolvedApp, a admission) []policyVerdict {
	t.Helper()
	rendered, err := datapolicies.Render(plan, resolved, nil)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	newObj, oldObj := &unstructured.Unstructured{Object: a.object}, &unstructured.Unstructured{Object: a.oldObject}
	attr := k8sadmission.NewAttributesRecord(newObj, oldObj, a.gvk, acceptNamespace, a.name, a.gvr, a.sub,
		k8sadmission.Operation(a.op), nil, false, &user.DefaultInfo{Name: a.user, Groups: a.groups})
	request := admissionv1.AdmissionRequest{
		UID: "uid", Kind: metav1.GroupVersionKind(a.gvk), Resource: metav1.GroupVersionResource(a.gvr),
		SubResource: a.sub, Name: a.name, Namespace: acceptNamespace, Operation: a.op,
		UserInfo: authenticationv1.UserInfo{Username: a.user, Groups: a.groups},
	}
	request.Object.Raw, request.OldObject.Raw = toJSON(t, a.object), toJSON(t, a.oldObject)
	data := map[string]any{
		compiler.NamespaceObjectKey: map[string]any{keyMetadata: map[string]any{keyName: acceptNamespace}},
		compiler.ObjectKey:          resolveVal(t, newObj),
		compiler.OldObjectKey:       resolveVal(t, oldObj),
		compiler.RequestKey:         resolveVal(t, &request),
	}
	var out []policyVerdict
	for i := range rendered {
		matched, err := matching.NewMatcher().Match(&matching.MatchCriteria{Constraints: rendered[i].Spec.MatchConstraints}, attr, nil)
		if err != nil {
			t.Fatalf("match: %v", err)
		}
		if rendered[i].Namespace == acceptNamespace && matched {
			out = append(out, compileVpol(t, &rendered[i]).evaluate(t, maps.Clone(data)))
		}
	}
	return out
}

func resolveVal(t *testing.T, obj any) any {
	t.Helper()
	val, err := celutils.ConvertObjectToUnstructured(obj)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	return val.Object
}

// A CEL runtime error denies in Enforce, so any error fails the test instead of giving a verdict.
func (p compiledPolicy) evaluate(t *testing.T, data map[string]any) policyVerdict {
	t.Helper()
	verdict := policyVerdict{status: statusSkip, actions: p.policy.Spec.ValidationAction}
	for _, condition := range p.matchConditions {
		if evalBool(t, condition, data) != types.True {
			return verdict
		}
	}
	vars := lazy.NewMapValue(compiler.VariablesType)
	data[compiler.VariablesKey] = vars
	for name, program := range p.variables {
		vars.Append(name, func(*lazy.MapValue) ref.Val {
			out, _, err := program.ContextEval(context.Background(), data)
			if err != nil {
				return types.WrapErr(err)
			}
			return out
		})
	}
	verdict.status = statusPass
	for _, v := range p.validations {
		if evalBool(t, v.Program, data) != types.False {
			continue
		}
		verdict.status, verdict.message = statusFail, v.Message
		if v.MessageExpression != nil {
			verdict.message = evalMessage(t, v.MessageExpression, data)
		}
		return verdict
	}
	return verdict
}

func evalBool(t *testing.T, program cel.Program, data map[string]any) ref.Val {
	t.Helper()
	out, _, err := program.ContextEval(context.Background(), data)
	if err != nil {
		t.Fatalf("CEL errored: %v", err)
	}
	return out
}

// Kyverno prints an expression's error in place of the message, so it must never fail.
func evalMessage(t *testing.T, program cel.Program, data map[string]any) string {
	t.Helper()
	out, _, err := program.ContextEval(context.Background(), data)
	if err != nil {
		t.Fatalf("message expression failed: %v", err)
	}
	message, ok := out.Value().(string)
	if !ok {
		t.Fatalf("message expression gave %T, want a string", out.Value())
	}
	return message
}

func verdictOf(verdicts []policyVerdict) string {
	for _, v := range verdicts {
		if v.status == statusFail {
			return verdictDeny
		}
	}
	return verdictAdmit
}

func mountSpec(secret string, extra ...map[string]any) map[string]any {
	app := container("app", "app:1")
	secretRef := map[string]any{"secretKeyRef": map[string]any{keyName: secret, "key": "k"}}
	app["env"] = []any{map[string]any{keyName: "K", "valueFrom": secretRef}}
	spec := podSpec(append([]map[string]any{app}, extra...)...)
	spec[keyVolumes] = []any{map[string]any{keyName: "cfg", "configMap": map[string]any{keyName: "cm"}}}
	return spec
}

func withVolumeItems(spec map[string]any) map[string]any {
	items := []any{map[string]any{"key": "a", "path": "a"}}
	spec[keyVolumes] = []any{map[string]any{keyName: "cfg", "configMap": map[string]any{keyName: "cm", "items": items}}}
	return spec
}

func withEmptyDir(spec map[string]any) map[string]any {
	spec[keyVolumes] = []any{map[string]any{keyName: "data", "emptyDir": map[string]any{}}}
	return spec
}

func withInit(spec map[string]any, initImage string) map[string]any {
	spec["initContainers"] = []any{container("init", initImage)}
	return spec
}

func withImage(reference string) map[string]any {
	return podSpec(container("c", reference))
}

func inNamespace(name, template string, a admission, want string) behaviorCase {
	return behaviorCase{name: name, template: template, scope: plans.ScopeTypeNamespaces, req: a, want: want}
}

func inApplication(name, template string, a admission, want string) behaviorCase {
	return behaviorCase{name: name, template: template, scope: plans.ScopeTypeApplications, req: a, want: want}
}

func excluding(c behaviorCase) behaviorCase {
	c.excluded = true
	return c
}

var behaviorCases = []behaviorCase{
	inNamespace("human delete", tplDelete, remove(gvkDeployment, gvrDeployment, otherName), verdictDeny),
	inNamespace("controller replaces a pod", tplDelete, remove(gvkPod, gvrPod, otherName).by(controllerManager), verdictAdmit),
	inNamespace("kube-system account deletes a replica set", tplDelete,
		remove(gvkReplicaSet, gvrReplicaSet, otherName).by(kubeSystemSA, kubeSystemGroup), verdictAdmit),
	inNamespace("policy write", tplDelete, remove(gvkPolicy, gvrPolicy, otherName), verdictAdmit),
	inNamespace("legacy policy cleanup", tplDelete, remove(gvkLegacy, gvrLegacy, otherName), verdictAdmit),
	inNamespace("report write", tplDelete, remove(gvkReport, gvrReport, otherName), verdictAdmit),
	inApplication("app member delete", tplDelete, remove(gvkDeployment, gvrDeployment, acceptApp), verdictDeny),
	inApplication("non-member delete", tplDelete, remove(gvkDeployment, gvrDeployment, otherName), verdictAdmit),
	excluding(inApplication("named exclusion", tplDelete, remove(gvkDeployment, gvrDeployment, acceptApp), verdictAdmit)),
	excluding(inApplication("kind exclusion", tplDelete, remove(gvkConfigMap, gvrConfigMap, acceptApp), verdictAdmit)),
	excluding(inApplication("outside the exclusions", tplDelete, remove(gvkSecret, gvrSecret, acceptApp), verdictDeny)),

	inNamespace("human update", tplUpdate, updateDeployment(podSpec(), podSpec(), replicasThree), verdictDeny),
	inNamespace("rollout resizes a replica set", tplUpdate, relabel(gvkReplicaSet, gvrReplicaSet, otherName).by(controllerManager), verdictAdmit),
	inNamespace("autoscaler scales through /scale", tplUpdate, scaleTo(gvrDeployment, otherName, replicasThree).by(controllerManager), verdictDeny),
	excluding(inNamespace("excluded kind update", tplUpdate, updateDeployment(podSpec(), podSpec(), replicasThree), verdictAdmit)),
	excluding(inNamespace("excluded kind keeps its /scale", tplUpdate, scaleTo(gvrDeployment, otherName, replicasThree), verdictAdmit)),
	excluding(inNamespace("other kind /scale", tplUpdate, scaleTo(gvrStatefulSet, otherName, replicasThree), verdictDeny)),
	inApplication("app member /scale", tplUpdate, scaleTo(gvrStatefulSet, acceptApp, replicasThree), verdictDeny),

	inNamespace("human create", tplCreate, createNamed(gvkConfigMap, gvrConfigMap, nil), verdictDeny),
	inNamespace("controller creates a pod", tplCreate, createNamed(gvkPod, gvrPod, nil).by(controllerManager), verdictAdmit),
	inApplication("new resource joins the app", tplCreate, createNamed(gvkConfigMap, gvrConfigMap, identity), verdictDeny),
	inApplication("deleted member re-created", tplCreate,
		create(gvkDeployment, gvrDeployment, deployment(acceptApp, replicasOne, podSpec())), verdictDeny),
	inApplication("unrelated create", tplCreate, createNamed(gvkConfigMap, gvrConfigMap, nil), verdictAdmit),
	inApplication("replacement pod keeps the label", tplCreate, createNamed(gvkPod, gvrPod, identity).by(controllerManager), verdictAdmit),

	inNamespace("blocked pattern", tplImageTypes, createDeployment(withImage("nginx:1.25")), verdictDeny),
	inNamespace("other image", tplImageTypes, createDeployment(withImage("redis:7")), verdictAdmit),
	inNamespace("? matches one character", tplImageTypes, createDeployment(withImage("registry.example.com/debug:1")), verdictDeny),
	inNamespace("a pattern's dot is literal", tplImageTypes, createDeployment(withImage("registryXexample.com/debug:1")), verdictAdmit),
	inNamespace("cronjob path", tplImageTypes, createCronJob(withImage("nginx")), verdictDeny),
	inNamespace("init container", tplImageTypes, createDeployment(withInit(withImage("app:1"), "nginx")), verdictDeny),
	inNamespace("already violating", tplImageTypes, updateDeployment(withImage("nginx"), withImage("nginx"), replicasThree), verdictDeny),

	inNamespace("untagged reads latest", tplImageTags, createDeployment(withImage("nginx")), verdictDeny),
	inNamespace("other tag", tplImageTags, createDeployment(withImage("nginx:1.25")), verdictAdmit),
	inNamespace("digest only", tplImageTags, createDeployment(withImage(digestImage)), verdictAdmit),
	inNamespace("cronjob tag", tplImageTags, createCronJob(withImage("busybox:dev")), verdictDeny),
	inNamespace("init container tag", tplImageTags, createDeployment(withInit(withImage("app:1"), "alpine:latest")), verdictDeny),

	inNamespace("replica change", tplReplicas, updateDeployment(podSpec(), podSpec(), replicasThree), verdictDeny),
	inNamespace("same replicas", tplReplicas, updateDeployment(podSpec(), withImage("a"), replicasOne), verdictAdmit),
	inNamespace("autoscaler /scale", tplReplicas, scaleTo(gvrDeployment, otherName, replicasThree).by(controllerManager), verdictDeny),
	inNamespace("unchanged /scale", tplReplicas, scaleTo(gvrDeployment, otherName, replicasOne), verdictAdmit),

	inNamespace("human claim update", tplStorage, relabel(gvkPVC, gvrPVC, otherName), verdictDeny),
	inNamespace("controller binds a claim", tplStorage, relabel(gvkPVC, gvrPVC, otherName).by(controllerManager), verdictAdmit),
	inNamespace("volume change", tplStorage, updateDeployment(podSpec(), withEmptyDir(podSpec()), replicasOne), verdictDeny),
	inNamespace("volumes unchanged", tplStorage, updateDeployment(podSpec(), podSpec(), replicasThree), verdictAdmit),
	inApplication("claim template wildcard", tplStorage, relabel(gvkPVC, gvrPVC, claimName), verdictDeny),
	inApplication("claim outside the app", tplStorage, relabel(gvkPVC, gvrPVC, otherName), verdictAdmit),

	inNamespace("config change", tplConfigSecret, relabel(gvkConfigMap, gvrConfigMap, otherName), verdictDeny),
	inNamespace("config create", tplConfigSecret, createNamed(gvkConfigMap, gvrConfigMap, nil), verdictAdmit),

	inNamespace("mounted config items change", tplMounts,
		updateDeployment(mountSpec("s"), withVolumeItems(mountSpec("s")), replicasOne), verdictDeny),
	inNamespace("env secret ref change", tplMounts, updateDeployment(mountSpec("s"), mountSpec("s2"), replicasOne), verdictDeny),
	inNamespace("sidecar without config", tplMounts,
		updateDeployment(mountSpec("s"), mountSpec("s", container("side", "side:1")), replicasOne), verdictAdmit),
	inNamespace("replicas only", tplMounts, updateDeployment(mountSpec("s"), mountSpec("s"), replicasThree), verdictAdmit),
}

func TestKyvernoJudgesEveryTemplate(t *testing.T) {
	for _, c := range behaviorCases {
		t.Run(c.template+"/"+c.name, func(t *testing.T) {
			testutil.Equal(t, "verdict", verdictOf(handle(t, c.template, c.scope, plans.ModeEnforce, c.excluded, c.req)), c.want)
		})
	}
}

// object is empty on DELETE: a template that reads it must not error, which would deny in Enforce.
func TestDeleteNeverErrors(t *testing.T) {
	refused := map[string]map[string]bool{
		tplDelete:       {"Deployment": true, "ConfigMap": true, "PersistentVolumeClaim": true},
		tplConfigSecret: {"ConfigMap": true},
		tplStorage:      {"PersistentVolumeClaim": true},
	}
	deletes := []admission{
		remove(gvkDeployment, gvrDeployment, otherName),
		remove(gvkConfigMap, gvrConfigMap, otherName),
		remove(gvkPVC, gvrPVC, otherName),
	}
	for _, tpl := range plans.Templates {
		for _, a := range deletes {
			t.Run(tpl.ID+"/"+a.gvk.Kind, func(t *testing.T) {
				want := verdictAdmit
				if refused[tpl.ID][a.gvk.Kind] {
					want = verdictDeny
				}
				testutil.Equal(t, "verdict", verdictOf(handle(t, tpl.ID, plans.ScopeTypeNamespaces, plans.ModeEnforce, false, a)), want)
			})
		}
	}
}

// An Audit plan records the same refusal without denying: Kyverno only blocks on the Deny action.
func TestAuditRecordsWithoutDeny(t *testing.T) {
	for mode, want := range map[string]admissionregistrationv1.ValidationAction{
		plans.ModeEnforce: admissionregistrationv1.Deny,
		plans.ModeAudit:   admissionregistrationv1.Audit,
	} {
		verdicts := handle(t, tplDelete, plans.ScopeTypeNamespaces, mode, false, remove(gvkDeployment, gvrDeployment, otherName))
		testutil.Equal(t, mode+" verdict", verdictOf(verdicts), verdictDeny)
		for _, v := range verdicts {
			testutil.Equal(t, mode+" action", slices.Equal(v.actions, []admissionregistrationv1.ValidationAction{want}), true)
		}
	}
}

// Kyverno's denial and its Event carry the plan's name and the requester as typed, and the
// violations reader gets both back exactly, however hostile.
func TestRefusalNamesThePlanAndTheRequester(t *testing.T) {
	for _, username := range []string{human, `x" + string(object) + "`, `back\slash`, `admin (requested by "z")`} {
		plan, resolved := acceptPlan(tplDelete, plans.ScopeTypeNamespaces, plans.ModeEnforce, false)
		plan.Name = `{{request.object}} "quoted"`
		rendered, err := datapolicies.Render(plan, resolved, nil)
		if err != nil || len(rendered) != constants.DefaultAddValue {
			t.Fatalf("render: %d policies, %v", len(rendered), err)
		}
		verdicts := handlePlan(t, plan, resolved, remove(gvkDeployment, gvrDeployment, otherName).by(username))
		if len(verdicts) != constants.DefaultAddValue || verdicts[constants.DefaultInitValue].status != statusFail {
			t.Fatalf("verdicts = %+v, want one refusal", verdicts)
		}
		policy := &rendered[constants.DefaultInitValue]
		event := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1", "kind": "Event", keyMetadata: map[string]any{keyName: "evt", keyNamespace: acceptNamespace},
			"reason":         "PolicyViolation",
			"message":        "Deployment " + acceptNamespace + "/" + otherName + ": fail (blocked); " + verdicts[constants.DefaultInitValue].message,
			"involvedObject": map[string]any{keyName: policy.Name, keyNamespace: acceptNamespace},
		}}
		dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
			map[schema.GroupVersionResource]string{{Version: "v1", Resource: "events"}: "EventList"}, event)
		got, err := violations.Collect(context.Background(), dyn, []string{acceptNamespace}, []string{policy.Name}, constants.EmptyString)
		if err != nil || len(got) != constants.DefaultAddValue {
			t.Fatalf("collect: %d rows, %v", len(got), err)
		}
		row := got[constants.DefaultInitValue]
		testutil.Equal(t, "requested by", row.RequestedBy, username)
		testutil.Equal(t, "message", row.Message, policy.Spec.Validations[constants.DefaultInitValue].Message)
		testutil.Equal(t, "names the plan", strings.Contains(row.Message, strconv.Quote(plan.Name)), true)
	}
}
