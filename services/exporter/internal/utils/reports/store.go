package reports

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/utils/artifact"
	reportseps "github.com/telark/rest/endpoints/reports"
)

const (
	metaKey  = "meta"
	tempGlob = "*" + constants.SnapshotTempFileSuffix
)

func PlanDir(root, planID string) string {
	return filepath.Join(root, constants.ReportsPlansSubdir, planID)
}

func ReportPath(root, planID, reportID string) string {
	return filepath.Join(PlanDir(root, planID), constants.ReportsReportsSubdir, reportID+constants.ReportFileExtension)
}

func LedgerPath(root, planID string) string {
	return filepath.Join(PlanDir(root, planID), constants.ReportsLedgerFile)
}

// Token-streams to the meta value and stops: the files member can be tens of
// megabytes and List never needs it.
func ReadMeta(r io.Reader) (reportseps.ReportMeta, error) {
	var meta reportseps.ReportMeta
	dec := json.NewDecoder(r)
	if err := expectToken(dec, json.Delim('{')); err != nil {
		return meta, err
	}
	if err := expectToken(dec, metaKey); err != nil {
		return meta, err
	}
	err := dec.Decode(&meta)
	return meta, err
}

func expectToken(dec *json.Decoder, want json.Token) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if tok != want {
		return fmt.Errorf("%w: got %v, want %v", ErrInvalidLedger, tok, want)
	}
	return nil
}

func validFiles(files map[string]string) bool {
	if len(files) != len(reportseps.Formats) {
		return false
	}
	for format := range files {
		if !slices.Contains(reportseps.Formats, format) {
			return false
		}
	}
	return true
}

func Create(root string, req reportseps.CreatePlanReportRequest) (reportseps.ReportMeta, bool, error) {
	var empty reportseps.ReportMeta
	if !artifact.IsSafeSegment(req.Meta.PlanID) || !artifact.IsSafeSegment(req.Meta.ID) {
		return empty, false, ErrBadID
	}
	if !validFiles(req.Files) {
		return empty, false, ErrBadFormat
	}
	path := ReportPath(root, req.Meta.PlanID, req.Meta.ID)
	if !artifact.IsWithinBase(path, root) {
		return empty, false, ErrBadID
	}
	if meta, err := readMetaFile(path); err == nil {
		return meta, true, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), constants.SnapshotDirPerm); err != nil {
		return empty, false, err
	}
	stage, err := artifact.WriteAtomic(path, func(w io.Writer) error {
		return json.NewEncoder(w).Encode(StoredReport{Meta: req.Meta, Files: req.Files})
	})
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrReportWriteFailed), stage, req.Meta.ID, path, err))
		return empty, false, err
	}
	prune(root, req.Meta.PlanID)
	return req.Meta, false, nil
}

func readMetaFile(path string) (reportseps.ReportMeta, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return reportseps.ReportMeta{}, err
	}
	defer func() { _ = f.Close() }()
	return ReadMeta(f)
}

// Boundary reports (end/cancel) are never pruned; only the newest manual ones
// survive.
func prune(root, planID string) {
	metas, err := List(root, planID)
	if err != nil {
		return
	}
	manual := slices.DeleteFunc(metas, func(m reportseps.ReportMeta) bool { return m.Trigger != reportseps.TriggerManual })
	for i := constants.ReportsMaxPerPlan; i < len(manual); i++ {
		path := ReportPath(root, planID, manual[i].ID)
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			lg.Error(fmt.Sprintf(string(constants.ErrReportWriteFailed), artifact.StageNone, manual[i].ID, path, err))
		}
	}
}

func List(root, planID string) ([]reportseps.ReportMeta, error) {
	entries, err := os.ReadDir(filepath.Join(PlanDir(root, planID), constants.ReportsReportsSubdir))
	if errors.Is(err, os.ErrNotExist) {
		return []reportseps.ReportMeta{}, nil
	}
	if err != nil {
		return nil, err
	}
	metas := make([]reportseps.ReportMeta, constants.DefaultInitValue, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), constants.ReportFileExtension) {
			continue
		}
		meta, err := readMetaFile(filepath.Join(PlanDir(root, planID), constants.ReportsReportsSubdir, entry.Name()))
		if err != nil {
			continue
		}
		metas = append(metas, meta)
	}
	slices.SortFunc(metas, newestFirst)
	return metas, nil
}

// GeneratedAt is always UTC RFC3339, so the string order is the time order.
func newestFirst(a, b reportseps.ReportMeta) int {
	return strings.Compare(b.GeneratedAt, a.GeneratedAt)
}

func ParseListFilter(query url.Values) (ListFilter, error) {
	filter := ListFilter{Trigger: query.Get(reportseps.QueryTrigger), Limit: constants.ReportsListDefaultLimit}
	for _, raw := range query[reportseps.QueryPlanID] {
		for id := range strings.SplitSeq(raw, constants.ReportsListSeparator) {
			if id = strings.TrimSpace(id); id != constants.EmptyString {
				filter.PlanIDs = append(filter.PlanIDs, id)
			}
		}
	}
	if filter.Trigger != constants.EmptyString && !slices.Contains(reportseps.Triggers, filter.Trigger) {
		return filter, ErrBadFilter
	}
	var err error
	if filter.From, err = parseBound(query.Get(reportseps.QueryFrom)); err != nil {
		return filter, err
	}
	if filter.To, err = parseBound(query.Get(reportseps.QueryTo)); err != nil {
		return filter, err
	}
	if raw := query.Get(reportseps.QueryLimit); raw != constants.EmptyString {
		limit, convErr := strconv.Atoi(raw)
		if convErr != nil || limit < constants.DefaultIncrementValue {
			return filter, ErrBadFilter
		}
		filter.Limit = min(limit, constants.ReportsListMaxLimit)
	}
	return filter, nil
}

func parseBound(raw string) (time.Time, error) {
	if raw == constants.EmptyString {
		return time.Time{}, nil
	}
	at, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, ErrBadFilter
	}
	return at, nil
}

func (f ListFilter) excludes(meta reportseps.ReportMeta) bool {
	if f.Trigger != constants.EmptyString && meta.Trigger != f.Trigger {
		return true
	}
	if f.From.IsZero() && f.To.IsZero() {
		return false
	}
	at, err := time.Parse(time.RFC3339, meta.GeneratedAt)
	return err != nil || (!f.From.IsZero() && at.Before(f.From)) || (!f.To.IsZero() && at.After(f.To))
}

// Every plan goes through List's meta-only reads, so a plan removed mid-walk
// reads as empty. Returns the newest filter.Limit matches and the match count.
func ListAll(root string, filter ListFilter) ([]reportseps.ReportMeta, int, error) {
	entries, err := os.ReadDir(filepath.Join(root, constants.ReportsPlansSubdir))
	if errors.Is(err, os.ErrNotExist) {
		return []reportseps.ReportMeta{}, constants.DefaultInitValue, nil
	}
	if err != nil {
		return nil, constants.DefaultInitValue, err
	}
	metas := []reportseps.ReportMeta{}
	for _, entry := range entries {
		planID := entry.Name()
		if !entry.IsDir() || !artifact.IsSafeSegment(planID) ||
			(len(filter.PlanIDs) > constants.DefaultInitValue && !slices.Contains(filter.PlanIDs, planID)) {
			continue
		}
		planMetas, listErr := List(root, planID)
		if listErr != nil {
			continue
		}
		metas = append(metas, slices.DeleteFunc(planMetas, filter.excludes)...)
	}
	slices.SortFunc(metas, newestFirst)
	total := len(metas)
	return metas[:min(total, filter.Limit)], total, nil
}

func Load(root, planID, reportID, format string) ([]byte, string, error) {
	if !artifact.IsSafeSegment(planID) || !artifact.IsSafeSegment(reportID) {
		return nil, constants.EmptyString, ErrBadID
	}
	if !slices.Contains(reportseps.Formats, format) {
		return nil, constants.EmptyString, ErrBadFormat
	}
	path := ReportPath(root, planID, reportID)
	if !artifact.IsWithinBase(path, root) {
		return nil, constants.EmptyString, ErrBadID
	}
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, constants.EmptyString, err
	}
	var stored StoredReport
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, constants.EmptyString, err
	}
	body, ok := stored.Files[format]
	if !ok {
		return nil, constants.EmptyString, os.ErrNotExist
	}
	return []byte(body), constants.ReportContentTypes[format], nil
}

func PutLedger(root, planID string, body []byte) error {
	if !artifact.IsSafeSegment(planID) {
		return ErrBadID
	}
	if !json.Valid(body) {
		return ErrInvalidLedger
	}
	path := LedgerPath(root, planID)
	if !artifact.IsWithinBase(path, root) {
		return ErrBadID
	}
	if err := os.MkdirAll(filepath.Dir(path), constants.SnapshotDirPerm); err != nil {
		return err
	}
	stage, err := artifact.WriteAtomic(path, func(w io.Writer) error {
		_, werr := w.Write(body)
		return werr
	})
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrReportWriteFailed), stage, planID, path, err))
	}
	return err
}

func GetLedger(root, planID string) ([]byte, error) {
	if !artifact.IsSafeSegment(planID) {
		return nil, ErrBadID
	}
	path := LedgerPath(root, planID)
	if !artifact.IsWithinBase(path, root) {
		return nil, ErrBadID
	}
	return os.ReadFile(filepath.Clean(path))
}

func RemovePlanReports(root, planID string) error {
	if !artifact.IsSafeSegment(planID) {
		return ErrBadID
	}
	dir := PlanDir(root, planID)
	if !artifact.IsWithinBase(dir, root) {
		return ErrBadID
	}
	if err := os.RemoveAll(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Never touches root or root/plans themselves: an unreferenced plan directory
// past the age gate goes whole, a live one only loses its stale temp files.
func SweepOrphans(root string, live map[string]struct{}, minAge time.Duration) (removed, temps, scanned int) {
	entries, err := os.ReadDir(filepath.Join(root, constants.ReportsPlansSubdir))
	if err != nil {
		return removed, temps, scanned
	}
	cutoff := time.Now().Add(-minAge)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		scanned++
		dir := PlanDir(root, entry.Name())
		if _, isLive := live[entry.Name()]; isLive {
			temps += removeOldTemps(dir, cutoff)
			continue
		}
		if info, statErr := entry.Info(); statErr == nil && info.ModTime().Before(cutoff) && os.RemoveAll(dir) == nil {
			removed++
		}
	}
	return removed, temps, scanned
}

func removeOldTemps(dir string, cutoff time.Time) int {
	removed := constants.DefaultInitValue
	for _, pattern := range []string{
		filepath.Join(dir, tempGlob),
		filepath.Join(dir, constants.ReportsReportsSubdir, tempGlob),
	} {
		matches, _ := filepath.Glob(pattern)
		for _, match := range matches {
			if info, err := os.Stat(match); err == nil && info.ModTime().Before(cutoff) && os.Remove(match) == nil {
				removed++
			}
		}
	}
	return removed
}
