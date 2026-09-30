package manifestdiff

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/telark/telark/internal/data/resources/application"
	kcoremanifest "github.com/telark/telark/internal/kcore/manifest"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/applications/history/changes"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Fingerprint covers exactly the roots Changes walks, so a status or
// managedFields write leaves it untouched and two objects sharing it diff empty.
func Fingerprint(u *unstructured.Unstructured) string {
	if u == nil {
		return constants.EmptyString
	}
	roots := diffRoots(ManifestPair{Old: u, New: u})
	compared := make([]any, len(roots))
	for i := range roots {
		compared[i] = roots[i].newValue
	}
	raw, err := json.Marshal(compared)
	if err != nil {
		return constants.EmptyString
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// Changes reports every spec, data, label and annotation difference between
// the two sides of each pair, for any kind and any field, so a change the
// curated summary checks do not know about still lands in the history.
func Changes(pairs []ManifestPair) []application.ApplicationChange {
	var out []application.ApplicationChange
	for i := range pairs {
		p := pairs[i]
		if p.Old == nil || p.New == nil {
			continue
		}
		kind, target := p.New.GetKind(), changeTarget(p.New)
		var rows []diffRow
		for _, root := range diffRoots(p) {
			walk(root.oldValue, root.newValue, root.path, &rows)
		}
		for _, r := range rows {
			if isCuratedPath(kind, r.path) {
				continue
			}
			if kind == kindSecret {
				redactSecretRow(&r)
			}
			out = append(out, toChange(target, r))
		}
	}
	return out
}

type diffRoot struct {
	path     string
	oldValue any
	newValue any
}

func diffRoots(p ManifestPair) []diffRoot {
	oldObj, newObj := applyClean(p.Old), applyClean(p.New)
	oldMeta := objectMap(oldObj[rootMetadata])
	newMeta := objectMap(newObj[rootMetadata])
	return []diffRoot{
		{rootSpec, oldObj[rootSpec], newObj[rootSpec]},
		{rootData, oldObj[rootData], newObj[rootData]},
		{pathLabels, metaMap(oldMeta, labelsKey), metaMap(newMeta, labelsKey)},
		{
			pathAnnotation,
			withoutNoisyAnnotations(metaMap(oldMeta, annotationsKey)),
			withoutNoisyAnnotations(metaMap(newMeta, annotationsKey)),
		},
	}
}

// A snapshot is the apply-clean copy kcore writes while a live informer object is raw; cleaning
// both sides keeps a snapshot laid back as a pre-image from listing every stripped field as added.
func applyClean(u *unstructured.Unstructured) map[string]any {
	obj := u.DeepCopy().Object
	kcoremanifest.CleanManifestForApply(obj)
	if u.GetKind() == kindService {
		if spec := objectMap(obj[rootSpec]); spec != nil {
			for _, f := range servedServiceSpecStripped {
				delete(spec, f)
			}
		}
	}
	return obj
}

// Roots returns the values Fingerprint hashes, keyed by path, for a stored
// copy that WithRoots lays back over a live object.
func Roots(u *unstructured.Unstructured) map[string]any {
	roots := diffRoots(ManifestPair{Old: u, New: u})
	out := make(map[string]any, len(roots))
	for i := range roots {
		if roots[i].newValue != nil {
			out[roots[i].path] = roots[i].newValue
		}
	}
	return out
}

// WithRoots returns a copy of live whose compared roots are roots; kind, name,
// namespace, status and the noisy annotations stay live's, so the result diffs
// against live as the object the roots came from would and still snapshots whole.
func WithRoots(live *unstructured.Unstructured, roots map[string]any) *unstructured.Unstructured {
	out := live.DeepCopy()
	putRoot(out.Object, rootSpec, roots[rootSpec])
	putRoot(out.Object, rootData, roots[rootData])
	meta := objectMap(out.Object[rootMetadata])
	if meta == nil {
		meta = make(map[string]any)
		out.Object[rootMetadata] = meta
	}
	putRoot(meta, labelsKey, roots[pathLabels])
	putRoot(meta, annotationsKey, withNoisyAnnotations(meta[annotationsKey], roots[pathAnnotation]))
	return out
}

func putRoot(m map[string]any, key string, v any) {
	if v == nil {
		delete(m, key)
		return
	}
	m[key] = v
}

func withNoisyAnnotations(live, stored any) any {
	out := make(map[string]any)
	for k, v := range objectMap(live) {
		if slices.Contains(noisyAnnotations, k) {
			out[k] = v
		}
	}
	maps.Copy(out, objectMap(stored))
	if len(out) == constants.DefaultInitValue {
		return nil
	}
	return out
}

func objectMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

// An empty map and an absent key compare equal: a stored copy loses the key
// once its last annotation is stripped, the live object keeps it as {}.
func metaMap(meta map[string]any, key string) any {
	if meta == nil {
		return nil
	}
	if m, ok := meta[key].(map[string]any); ok && len(m) == constants.DefaultInitValue {
		return nil
	}
	return meta[key]
}

func withoutNoisyAnnotations(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	out := make(map[string]any, len(m))
	for k, val := range m {
		if slices.Contains(noisyAnnotations, k) {
			continue
		}
		out[k] = val
	}
	if len(out) == constants.DefaultInitValue {
		return nil
	}
	return out
}

// A map appearing or vanishing is reported per key, the same rows a map that
// emptied would produce.
func emptyMapForNil(v, other any) any {
	if _, ok := other.(map[string]any); ok && v == nil {
		return map[string]any{}
	}
	return v
}

func walk(oldV, newV any, path string, out *[]diffRow) {
	if oldV == nil && newV == nil {
		return
	}
	oldV, newV = emptyMapForNil(oldV, newV), emptyMapForNil(newV, oldV)
	if oldV == nil {
		*out = append(*out, diffRow{path: path, changeType: changes.ChangeTypeAdded, newValue: stringify(newV)})
		return
	}
	if newV == nil {
		*out = append(*out, diffRow{path: path, changeType: changes.ChangeTypeRemoved, oldValue: stringify(oldV)})
		return
	}
	oldMap, oldIsMap := oldV.(map[string]any)
	newMap, newIsMap := newV.(map[string]any)
	if oldIsMap && newIsMap {
		walkMaps(oldMap, newMap, path, out)
		return
	}
	oldList, oldIsList := oldV.([]any)
	newList, newIsList := newV.([]any)
	if oldIsList && newIsList {
		walkLists(oldList, newList, path, out)
		return
	}
	if reflect.DeepEqual(oldV, newV) {
		return
	}
	*out = append(*out, diffRow{
		path: path, changeType: changes.ChangeTypeUpdated, oldValue: stringify(oldV), newValue: stringify(newV),
	})
}

func walkMaps(oldMap, newMap map[string]any, path string, out *[]diffRow) {
	keys := make(map[string]struct{}, len(oldMap)+len(newMap))
	for k := range oldMap {
		keys[k] = struct{}{}
	}
	for k := range newMap {
		keys[k] = struct{}{}
	}
	for _, k := range slices.Sorted(maps.Keys(keys)) {
		walk(oldMap[k], newMap[k], joinPath(path, k), out)
	}
}

// Lists of named objects (containers, ports, env, volumes) are matched by name so a reorder is
// not a change; other lists are compared positionally and reported whole when their length differs.
func walkLists(oldList, newList []any, path string, out *[]diffRow) {
	oldByName, oldNamed := namedItems(oldList)
	newByName, newNamed := namedItems(newList)
	if oldNamed && newNamed {
		names := make(map[string]struct{}, len(oldByName)+len(newByName))
		for n := range oldByName {
			names[n] = struct{}{}
		}
		for n := range newByName {
			names[n] = struct{}{}
		}
		for _, n := range slices.Sorted(maps.Keys(names)) {
			walk(oldByName[n], newByName[n], path+indexOpen+n+indexClose, out)
		}
		return
	}
	if len(oldList) != len(newList) {
		if !reflect.DeepEqual(oldList, newList) {
			*out = append(*out, diffRow{
				path: path, changeType: changes.ChangeTypeUpdated, oldValue: stringify(oldList), newValue: stringify(newList),
			})
		}
		return
	}
	for i := range oldList {
		walk(oldList[i], newList[i], fmt.Sprintf("%s%s%d%s", path, indexOpen, i, indexClose), out)
	}
}

func namedItems(list []any) (map[string]any, bool) {
	if len(list) == constants.DefaultInitValue {
		return nil, false
	}
	byName := make(map[string]any, len(list))
	for _, it := range list {
		m, ok := it.(map[string]any)
		if !ok {
			return nil, false
		}
		n, ok := m[itemNameKey].(string)
		if !ok || n == constants.EmptyString {
			return nil, false
		}
		byName[n] = m
	}
	return byName, true
}

func joinPath(path, key string) string {
	if path == constants.EmptyString {
		return key
	}
	return path + pathSeparator + key
}

func stringify(v any) *string {
	var s string
	switch t := v.(type) {
	case string:
		s = t
	case map[string]any, []any:
		b, err := json.Marshal(t)
		if err != nil {
			s = fmt.Sprint(t)
		} else {
			s = string(b)
		}
	default:
		s = fmt.Sprint(t)
	}
	if utf8.RuneCountInString(s) > maxValueRunes {
		s = string([]rune(s)[:maxValueRunes]) + ellipsis
	}
	return &s
}

func isCuratedPath(kind, path string) bool {
	if slices.Contains(curatedExactPaths, path) {
		return true
	}
	for _, suffix := range curatedPathSuffixes {
		if strings.HasSuffix(path, suffix) || strings.Contains(path, suffix+pathSeparator) {
			return true
		}
	}
	for _, p := range curatedKindPaths[kind] {
		if path == p || strings.HasPrefix(path, p+pathSeparator) || strings.HasPrefix(path, p+indexOpen) {
			return true
		}
	}
	return false
}

// An app spanning namespaces can hold the same Kind/name twice; the namespace tells them apart.
func changeTarget(u *unstructured.Unstructured) string {
	target := u.GetKind() + kindNameSep + u.GetName()
	if ns := u.GetNamespace(); ns != constants.EmptyString {
		return ns + kindNameSep + target
	}
	return target
}

func toChange(target string, r diffRow) application.ApplicationChange {
	var desc string
	switch r.changeType {
	case changes.ChangeTypeAdded:
		desc = target + descColon + r.path + descAdded + deref(r.newValue)
	case changes.ChangeTypeRemoved:
		desc = target + descColon + r.path + descRemoved + deref(r.oldValue)
	default:
		desc = target + descColon + r.path + descColon + deref(r.oldValue) + descTransition + deref(r.newValue)
	}
	return application.ApplicationChange{
		Field:       target + changes.ManifestFieldSeparator + r.path,
		Description: desc,
		ChangeType:  r.changeType,
		OldValue:    r.oldValue,
		NewValue:    r.newValue,
	}
}

func deref(s *string) string {
	if s == nil {
		return constants.EmptyString
	}
	return *s
}

// redactSecretRow keeps the fact that a Secret entry changed but never its value.
func redactSecretRow(r *diffRow) {
	if !strings.HasPrefix(r.path, rootData) && !strings.HasPrefix(r.path, rootStringData) {
		return
	}
	redacted := redactedValue
	if r.oldValue != nil {
		r.oldValue = &redacted
	}
	if r.newValue != nil {
		r.newValue = &redacted
	}
}
