package manifestdiff

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/applications/history/changes"
)

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
		kind, name := p.New.GetKind(), p.New.GetName()
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
			out = append(out, toChange(kind, name, r))
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
	oldMeta := objectMap(p.Old.Object[rootMetadata])
	newMeta := objectMap(p.New.Object[rootMetadata])
	return []diffRoot{
		{rootSpec, p.Old.Object[rootSpec], p.New.Object[rootSpec]},
		{rootData, p.Old.Object[rootData], p.New.Object[rootData]},
		{rootMetadata + pathSeparator + labelsKey, metaMap(oldMeta, labelsKey), metaMap(newMeta, labelsKey)},
		{
			rootMetadata + pathSeparator + annotationsKey,
			withoutNoisyAnnotations(metaMap(oldMeta, annotationsKey)),
			withoutNoisyAnnotations(metaMap(newMeta, annotationsKey)),
		},
	}
}

func objectMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func metaMap(meta map[string]any, key string) any {
	if meta == nil {
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
	return out
}

func walk(oldV, newV any, path string, out *[]diffRow) {
	if oldV == nil && newV == nil {
		return
	}
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
	for _, k := range slices.Sorted(mapKeys(keys)) {
		walk(oldMap[k], newMap[k], joinPath(path, k), out)
	}
}

func mapKeys(m map[string]struct{}) func(yield func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// walkLists diffs lists item by item. Lists of named objects (containers,
// ports, env, volumes) are matched by name so a reorder is not a change and a
// changed item keeps a stable path; other lists are compared positionally and
// reported whole when their length differs.
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
		for _, n := range slices.Sorted(mapKeys(names)) {
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

func toChange(kind, name string, r diffRow) application.ApplicationChange {
	target := kind + kindNameSep + name
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
