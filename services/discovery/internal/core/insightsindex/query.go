package insightsindex

import (
	"cmp"
	"fmt"
	"hash/fnv"
	"math"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/telark/discovery/internal/constants"
)

func ParseQuery(values url.Values) (Query, error) {
	q := Query{
		Category:    strings.TrimSpace(values.Get(ParamCategory)),
		Kinds:       parseList(values.Get(ParamKind)),
		Severities:  parseList(values.Get(ParamSeverity)),
		States:      parseList(values.Get(ParamState)),
		Triage:      strings.TrimSpace(values.Get(ParamTriage)),
		Namespaces:  parseList(values.Get(ParamNamespace)),
		Environment: strings.TrimSpace(values.Get(ParamEnvironment)),
		Search:      strings.ToLower(strings.TrimSpace(values.Get(ParamSearch))),
		IDs:         parseList(values.Get(ParamID)),
		Apps:        parseList(values.Get(ParamApp)),
	}
	var pageOK, sizeOK, freshOK bool
	q.Page, pageOK = parseInt(values.Get(ParamPage), DefaultPage, DefaultPage, math.MaxInt32)
	q.PageSize, sizeOK = parseInt(values.Get(ParamPageSize), DefaultPageSize, DefaultPage, MaxPageSize)
	q.Fresh, freshOK = parseBool(values.Get(ParamFresh))
	checks := []struct {
		param string
		ok    bool
	}{
		{ParamCategory, q.Category == constants.EmptyString || slices.Contains(categories, q.Category)},
		{ParamKind, subsetOf(q.Kinds, kinds)},
		{ParamSeverity, subsetOf(q.Severities, severities)},
		{ParamState, subsetOf(q.States, states)},
		{ParamTriage, q.Triage == constants.EmptyString || slices.Contains(triageFilters, q.Triage)},
		{ParamApp, validMembers(q.Apps)},
		{ParamPage, pageOK},
		{ParamPageSize, sizeOK},
		{ParamFresh, freshOK},
	}
	for _, check := range checks {
		if !check.ok {
			return Query{}, fmt.Errorf(string(constants.ErrInsightsQueryInvalid), check.param)
		}
	}
	if len(q.States) == constants.DefaultInitValue {
		q.States = defaultStates
	}
	return q, nil
}

func parseList(raw string) []string {
	var out []string
	for part := range strings.SplitSeq(raw, listSeparator) {
		if trimmed := strings.TrimSpace(part); trimmed != constants.EmptyString {
			out = append(out, trimmed)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func parseInt(raw string, fallback, low, high int) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == constants.EmptyString {
		return fallback, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < low || n > high {
		return fallback, false
	}
	return n, true
}

func parseBool(raw string) (value, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == constants.EmptyString {
		return false, true
	}
	value, err := strconv.ParseBool(raw)
	return value, err == nil
}

func validMembers(members []string) bool {
	return !slices.ContainsFunc(members, func(member string) bool {
		_, _, ok := splitMember(member)
		return !ok
	})
}

func subsetOf(values, allowed []string) bool {
	return !slices.ContainsFunc(values, func(v string) bool { return !slices.Contains(allowed, v) })
}

// Weak: the tag names this replica's view of the index, not a byte-exact body.
func (x *Index) ETag(q *Query, excluded []string, now time.Time) string {
	x.mu.RLock()
	defer x.mu.RUnlock()
	return x.tagFor(q, excluded, now.UnixMilli())
}

// Counts cover the whole filtered set; only items are paged.
func (x *Index) Query(q *Query, excluded []string, now time.Time) (Page, string) {
	x.mu.RLock()
	defer x.mu.RUnlock()
	nowMs := now.UnixMilli()
	counts := Counts{
		BySeverity: map[string]int{}, ByCategory: map[string]int{}, ByState: map[string]int{}, SeverityFacet: map[string]int{},
	}
	var matched []*Row
	for _, list := range x.rows {
		if len(list) == constants.DefaultInitValue || !q.appMatches(&list[0], excluded) {
			continue
		}
		for i := range list {
			row := &list[i]
			if !x.rowMatches(q, row, excluded) || !q.matchesIgnoringSeverity(row, nowMs) {
				continue
			}
			counts.SeverityFacet[row.Severity]++
			if !anyOf(q.Severities, row.Severity) {
				continue
			}
			counts.BySeverity[row.Severity]++
			counts.ByCategory[row.Category]++
			counts.ByState[row.state(nowMs)]++
			matched = append(matched, row)
		}
	}
	slices.SortFunc(matched, compareRows)
	items := x.items(q, matched, nowMs)
	return Page{
		Items:     items,
		Total:     len(matched),
		Page:      q.Page,
		PageSize:  q.PageSize,
		Counts:    counts,
		IndexedAt: x.indexedAt.UTC().Format(time.RFC3339),
	}, x.tagFor(q, excluded, nowMs)
}

func (q *Query) appMatches(row *Row, excluded []string) bool {
	return !slices.Contains(excluded, row.Namespace) && anyOf(q.Apps, row.Namespace+memberSeparator+row.App)
}

// Per row, not per app: a multi-namespace app's cards can be about workloads outside the document's namespace.
func (x *Index) rowMatches(q *Query, row *Row, excluded []string) bool {
	if slices.Contains(excluded, row.WorkloadNamespace) || !anyOf(q.Namespaces, row.WorkloadNamespace) {
		return false
	}
	return q.Environment == constants.EmptyString ||
		slices.Contains(x.envByApp[row.App], q.Environment) ||
		slices.Contains(x.envByNS[row.Namespace], q.Environment) ||
		slices.Contains(x.envByNS[row.WorkloadNamespace], q.Environment)
}

func (q *Query) matchesIgnoringSeverity(row *Row, nowMs int64) bool {
	return (q.Category == constants.EmptyString || row.Category == q.Category) &&
		anyOf(q.IDs, row.ID) &&
		anyOf(q.Kinds, row.Kind) &&
		anyOf(q.States, row.state(nowMs)) &&
		triageMatches(q.Triage, row) &&
		(q.Search == constants.EmptyString || strings.Contains(row.haystack, q.Search))
}

func anyOf(values []string, value string) bool {
	return len(values) == constants.DefaultInitValue || slices.Contains(values, value)
}

// The default view (no triage filter) hides dismissed rows; "all" shows them.
func triageMatches(filter string, row *Row) bool {
	switch filter {
	case TriageAll:
		return true
	case TriageUntriaged:
		return row.Triage == nil
	case constants.EmptyString:
		return row.Triage == nil || row.Triage.State != TriageDismissed
	default:
		return row.Triage != nil && row.Triage.State == filter
	}
}

func (r *Row) stale(nowMs int64) bool {
	return r.staleAtMs != constants.ZeroInt64 && r.staleAtMs < nowMs
}

func (r *Row) state(nowMs int64) string {
	if r.stale(nowMs) {
		return StateStale
	}
	return r.Status
}

// One fixed order, most severe then most recent first; the UI sorts the table itself. Ends on
// namespace, app and id so the order is total and pages never overlap.
func compareRows(a, b *Row) int {
	return cmp.Or(
		cmp.Compare(severityRank[b.Severity], severityRank[a.Severity]),
		cmp.Compare(b.lastSeenMs, a.lastSeenMs),
		cmp.Compare(a.Namespace, b.Namespace),
		cmp.Compare(a.App, b.App),
		cmp.Compare(a.ID, b.ID),
	)
}

func (x *Index) items(q *Query, matched []*Row, nowMs int64) []Row {
	start := min((q.Page-DefaultPage)*q.PageSize, len(matched))
	window := matched[start:min(start+q.PageSize, len(matched))]
	out := make([]Row, constants.DefaultInitValue, len(window))
	for _, row := range window {
		item := *row
		item.Stale = row.stale(nowMs)
		item.Environments = x.environments(row)
		out = append(out, item)
	}
	return out
}

func (x *Index) environments(row *Row) []string {
	byApp, byNS, byWorkloadNS := x.envByApp[row.App], x.envByNS[row.Namespace], x.envByNS[row.WorkloadNamespace]
	out := make([]string, constants.DefaultInitValue, len(byApp)+len(byNS)+len(byWorkloadNS))
	out = append(append(append(out, byApp...), byNS...), byWorkloadNS...)
	slices.Sort(out)
	return slices.Compact(out)
}

// Hashes the stale-deadline position too, so a row turning stale by time alone still changes the tag.
func (x *Index) tagFor(q *Query, excluded []string, nowMs int64) string {
	staleRows, _ := slices.BinarySearch(x.staleAtMs, nowMs)
	fields := []string{
		strconv.FormatInt(x.epoch, constants.IntBase10),
		q.Category,
		strings.Join(q.Kinds, listSeparator),
		strings.Join(q.Severities, listSeparator),
		strings.Join(q.States, listSeparator),
		q.Triage,
		strings.Join(q.Namespaces, listSeparator),
		q.Environment,
		q.Search,
		strings.Join(q.IDs, listSeparator),
		strings.Join(q.Apps, listSeparator),
		strconv.Itoa(q.Page),
		strconv.Itoa(q.PageSize),
		strings.Join(slices.Sorted(slices.Values(excluded)), listSeparator),
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(strings.Join(fields, etagFieldSeparator)))
	return fmt.Sprintf(etagFormat, x.version, staleRows, h.Sum64())
}
