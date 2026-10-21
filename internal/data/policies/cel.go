package policies

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/telark/telark/internal/data/constants"
	kcoreshared "github.com/telark/telark/internal/kcore/shared"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	celAnd        = " && "
	celOr         = " || "
	celIn         = " in "
	celNot        = "!"
	celTrue       = "true"
	celGroupFmt   = "(%s)"
	celListFmt    = "[%s]"
	celListSep    = ", "
	globChars     = "*?"
	globAnyRun    = `\*`
	globAnyOne    = `\?`
	regexAnyRun   = ".*"
	regexAnyOne   = "."
	regexAltSep   = "|"
	regexWholeFmt = "^(?:%s)$"
	ruleKeySep    = "|"

	exprKind          = "request.kind.kind"
	exprResource      = "request.resource.resource"
	exprSubResource   = `request.?subResource.orValue("")`
	exprName          = `request.?name.orValue("")`
	exprUsername      = `request.userInfo.?username.orValue("")`
	exprNoSubResource = exprSubResource + ` == ""`
	exprScaleFmt      = exprSubResource + ` == "scale" && ` + exprResource + celIn + "%s"
	exprGroupsFmt     = "request.userInfo.?groups.orValue([]).exists(g, g in %s)"
	exprAppIDFmt      = `object.?metadata.?labels[?%s].orValue("")`
	exprMatchesFmt    = "%s.matches(%s)"
	// Kyverno prints a failing message expression's error instead of the message, so this one
	// cannot fail: the username is quoted the way strconv.Unquote reads it back.
	exprRequestedBySuffix = ` + " (requested by \"" + ` + exprUsername +
		`.replace("\\", "\\\\").replace("\"", "\\\"") + "\")"`
	// A CronJob nests its pod template one level deeper than every other workload kind, so a
	// spec.template.spec read alone resolves to nothing on a CronJob and admits it.
	exprPodSpecFmt = `%[1]s == %[2]s ? %[3]s.?spec.?jobTemplate.?spec.?template.?spec.orValue({})` +
		` : %[3]s.?spec.?template.?spec.orValue({})`
	exprContainersFmt = `%[1]s.?containers.orValue([]) + %[1]s.?initContainers.orValue([])` +
		` + %[1]s.?ephemeralContainers.orValue([])`

	rootObject    = "object"
	rootOldObject = "oldObject"

	varPodSpec       = "podSpec"
	varOldPodSpec    = "oldPodSpec"
	varContainers    = "containers"
	varOldContainers = "oldContainers"
	variablesPrefix  = "variables."

	VarPodSpec       = variablesPrefix + varPodSpec
	VarOldPodSpec    = variablesPrefix + varOldPodSpec
	VarContainers    = variablesPrefix + varContainers
	VarOldContainers = variablesPrefix + varOldContainers

	groupApps                      = "apps"
	versionV1                      = "v1"
	kindReplicaSet                 = "ReplicaSet"
	kindReplicationController      = "ReplicationController"
	resourceReplicaSets            = "replicasets"
	resourceReplicationControllers = "replicationcontrollers"
)

// The kinds an application holds come from the list discovery reads applications from, so every
// resource it reports resolves; no application owns the two scale-only kinds.
var kindResources = func() map[string]schema.GroupVersionResource {
	out := map[string]schema.GroupVersionResource{
		kindReplicaSet:            {Group: groupApps, Version: versionV1, Resource: resourceReplicaSets},
		kindReplicationController: {Group: constants.EmptyString, Version: versionV1, Resource: resourceReplicationControllers},
	}
	for _, gvr := range kcoreshared.AppGVRs() {
		out[kcoreshared.ResourceKind(gvr.Resource)] = gvr
	}
	return out
}()

// Every literal reaches CEL through this one encoder: Go's escapes are a subset of CEL's, and
// the value decodes unchanged.
func CELString(s string) string {
	return strconv.Quote(s)
}

func CELList(values []string) string {
	quoted := make([]string, constants.DefaultInitValue, len(values))
	for _, v := range values {
		quoted = append(quoted, CELString(v))
	}
	return fmt.Sprintf(celListFmt, strings.Join(quoted, celListSep))
}

func celNegate(expr string) string {
	return celNot + fmt.Sprintf(celGroupFmt, expr)
}

// && binds tighter than ||, so only an || part needs brackets inside an && join.
func joinExpr(parts []string, op string) string {
	if len(parts) == constants.DefaultInitValue {
		return celTrue
	}
	out := make([]string, constants.DefaultInitValue, len(parts))
	for _, p := range parts {
		if op == celAnd && strings.Contains(p, celOr) {
			p = fmt.Sprintf(celGroupFmt, p)
		}
		out = append(out, p)
	}
	return strings.Join(out, op)
}

// Kyverno's wildcards as anchored RE2: * is any run, ? is one character.
func GlobRegex(globs []string) string {
	quoted := make([]string, constants.DefaultInitValue, len(globs))
	replacer := strings.NewReplacer(globAnyRun, regexAnyRun, globAnyOne, regexAnyOne)
	for _, g := range globs {
		quoted = append(quoted, replacer.Replace(regexp.QuoteMeta(g)))
	}
	return fmt.Sprintf(regexWholeFmt, strings.Join(quoted, regexAltSep))
}

// The controller exemption is stated once for all the branches that carry it.
func AnyOf(filters []ResourceFilter) string {
	var exempt, plain []string
	for _, f := range filters {
		if f.ExceptControllers {
			exempt = append(exempt, filterExpr(f))
			continue
		}
		plain = append(plain, filterExpr(f))
	}
	if len(exempt) == constants.DefaultInitValue {
		return joinExpr(plain, celOr)
	}
	exempted := joinExpr([]string{joinExpr(exempt, celOr), celNegate(controllersExpr)}, celAnd)
	return joinExpr(append([]string{exempted}, plain...), celOr)
}

func filterExpr(f ResourceFilter) string {
	var parts []string
	if len(f.Kinds) > constants.DefaultInitValue {
		parts = append(parts, kindExpr(f.Kinds))
	}
	if len(f.Names) > constants.DefaultInitValue {
		parts = append(parts, nameExpr(f.Names))
	}
	if len(f.AppIDs) > constants.DefaultInitValue {
		parts = append(parts, fmt.Sprintf(exprAppIDFmt, CELString(AppNameLabel))+celIn+CELList(f.AppIDs))
	}
	return joinExpr(parts, celAnd)
}

// A scale request carries kind Scale and the parent's resource, so `K/scale` is matched by
// resource; `*` never reaches a subresource, as Kyverno's `*` kind never did.
func kindExpr(kinds []string) string {
	var plain, scaled, parts []string
	for _, kind := range kinds {
		switch base := baseKind(kind); {
		case kind == KindWildcard:
			parts = append(parts, exprNoSubResource)
		case base != kind:
			if gvr, ok := kindResources[base]; ok {
				scaled = append(scaled, gvr.Resource)
			}
		default:
			plain = append(plain, kind)
		}
	}
	if len(plain) > constants.DefaultInitValue {
		parts = append(parts, exprKind+celIn+CELList(plain))
	}
	if len(scaled) > constants.DefaultInitValue {
		parts = append(parts, fmt.Sprintf(exprScaleFmt, CELList(scaled)))
	}
	return joinExpr(parts, celOr)
}

func nameExpr(names []string) string {
	var exact, globs []string
	for _, name := range names {
		if strings.ContainsAny(name, globChars) {
			globs = append(globs, name)
			continue
		}
		exact = append(exact, name)
	}
	var parts []string
	if len(exact) > constants.DefaultInitValue {
		parts = append(parts, exprName+celIn+CELList(exact))
	}
	if len(globs) > constants.DefaultInitValue {
		parts = append(parts, fmt.Sprintf(exprMatchesFmt, exprName, CELString(GlobRegex(globs))))
	}
	return joinExpr(parts, celOr)
}

var controllersExpr = func() string {
	var users, groups []string
	for _, s := range ControllerSubjects {
		if s.Kind == rbacv1.GroupKind {
			groups = append(groups, s.Name)
			continue
		}
		users = append(users, s.Name)
	}
	return exprUsername + celIn + CELList(users) + celOr + fmt.Sprintf(exprGroupsFmt, CELList(groups))
}()

var platformExpr = exprKind + celIn + CELList(PlatformKinds)

func PodSpecVariables() []admissionregistrationv1.Variable {
	return []admissionregistrationv1.Variable{
		{Name: varPodSpec, Expression: fmt.Sprintf(exprPodSpecFmt, exprKind, CELString(KindCronJob), rootObject)},
		{Name: varOldPodSpec, Expression: fmt.Sprintf(exprPodSpecFmt, exprKind, CELString(KindCronJob), rootOldObject)},
		{Name: varContainers, Expression: fmt.Sprintf(exprContainersFmt, VarPodSpec)},
		{Name: varOldContainers, Expression: fmt.Sprintf(exprContainersFmt, VarOldPodSpec)},
	}
}

// One rule per API group and operation set, `*` and `/scale` each in their own, so a request
// reaches the policy only for a resource and verb some filter names.
func resourceRules(filters []ResourceFilter) []admissionregistrationv1.NamedRuleWithOperations {
	groups := map[string]*ruleGroup{}
	for _, f := range filters {
		ops := strings.Join(f.Ops, ruleKeySep)
		for _, kind := range f.Kinds {
			group, resource, ok := ruleTarget(kind)
			if !ok {
				continue
			}
			key := strings.Join([]string{group, ops, strconv.FormatBool(baseKind(kind) != kind)}, ruleKeySep)
			g, seen := groups[key]
			if !seen {
				g = &ruleGroup{group: group, ops: f.Ops}
				groups[key] = g
			}
			g.resources = append(g.resources, resource)
		}
	}
	keys := slices.Sorted(maps.Keys(groups))
	out := make([]admissionregistrationv1.NamedRuleWithOperations, constants.DefaultInitValue, len(keys))
	for _, key := range keys {
		g := groups[key]
		out = append(out, admissionregistrationv1.NamedRuleWithOperations{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Operations: operationTypes(g.ops),
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{g.group},
					APIVersions: []string{KindWildcard},
					Resources:   slices.Compact(slices.Sorted(slices.Values(g.resources))),
				},
			},
		})
	}
	return out
}

// The `*` resource group is its own API group, so it never shares a rule with a named resource.
func ruleTarget(kind string) (group, resource string, ok bool) {
	if kind == KindWildcard {
		return KindWildcard, KindWildcard, true
	}
	base := baseKind(kind)
	gvr, ok := kindResources[base]
	if !ok {
		return constants.EmptyString, constants.EmptyString, false
	}
	if base != kind {
		return gvr.Group, gvr.Resource + scaleSuffix, true
	}
	return gvr.Group, gvr.Resource, true
}

func operationTypes(ops []string) []admissionregistrationv1.OperationType {
	out := make([]admissionregistrationv1.OperationType, constants.DefaultInitValue, len(ops))
	for _, op := range ops {
		out = append(out, admissionregistrationv1.OperationType(op))
	}
	return out
}
