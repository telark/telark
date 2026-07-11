package metrics

import (
	"math"
	"strings"
	"time"

	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
)

func computeDerivedMetrics(app *application.Application) application.DerivedMetrics {
	if app == nil {
		return emptyDerivedMetrics(constants.DefaultInitValue)
	}
	log := app.History.ChangeLog
	if log == nil {
		return emptyDerivedMetrics(len(app.Snapshots))
	}
	if len(log) == constants.DefaultInitValue {
		return emptyDerivedMetrics(len(app.Snapshots))
	}
	return derivedMetricsFromChangeLog(log, len(app.Snapshots))
}

func emptyDerivedMetrics(snapshotCount int) application.DerivedMetrics {
	return application.DerivedMetrics{
		TotalChanges:         constants.DefaultInitValue,
		ChangesByClass:       map[string]int{},
		ChangesBySeverity:    map[string]int{},
		SnapshotCount:        snapshotCount,
		ChangeVelocityPerDay: float64(constants.DefaultInitValue),
	}
}

func derivedMetricsFromChangeLog(
	log []application.ChangeLogEntry,
	snapshotCount int,
) application.DerivedMetrics {
	byClass := map[string]int{}
	bySeverity := map[string]int{}
	incidents := constants.DefaultInitValue
	recoveries := constants.DefaultInitValue
	fps := map[string]struct{}{}
	total := constants.DefaultInitValue

	for i := range log {
		e := log[i]
		if strings.TrimSpace(e.ChangeClass) != constants.EmptyString {
			byClass[e.ChangeClass]++
			total++
		}
		if strings.TrimSpace(e.Severity) != constants.EmptyString {
			bySeverity[e.Severity]++
		}
		if e.IsIncident {
			incidents++
		}
		if e.IsRecovery {
			recoveries++
		}
		if strings.TrimSpace(e.Fingerprint) != constants.EmptyString {
			fps[e.Fingerprint] = struct{}{}
		}
	}

	firstStr := log[constants.DefaultInitValue].DetectedAt
	lastIdx := len(log) - constants.DefaultAddValue
	lastStr := log[lastIdx].DetectedAt
	velocity := changeVelocityPerDay(firstStr, lastStr, total)

	var firstPtr, lastPtr *string
	if strings.TrimSpace(firstStr) != constants.EmptyString {
		firstPtr = &log[constants.DefaultInitValue].DetectedAt
	}
	if strings.TrimSpace(lastStr) != constants.EmptyString {
		lastPtr = &log[lastIdx].DetectedAt
	}

	return application.DerivedMetrics{
		TotalChanges:          total,
		ChangesByClass:        byClass,
		ChangesBySeverity:     bySeverity,
		TotalIncidents:        incidents,
		TotalRecoveries:       recoveries,
		SnapshotCount:         snapshotCount,
		FirstChangeDetectedAt: firstPtr,
		LastChangeDetectedAt:  lastPtr,
		ChangeVelocityPerDay:  velocity,
		UniqueFingerprints:    len(fps),
	}
}

func changeVelocityPerDay(firstStr, lastStr string, totalChanges int) float64 {
	firstStr = strings.TrimSpace(firstStr)
	lastStr = strings.TrimSpace(lastStr)
	empty := constants.EmptyString
	noChanges := totalChanges == constants.DefaultInitValue
	if firstStr == empty || lastStr == empty || noChanges {
		return float64(constants.DefaultInitValue)
	}
	firstT, err1 := time.Parse(time.RFC3339Nano, firstStr)
	if err1 != nil {
		firstT, err1 = time.Parse(time.RFC3339, firstStr)
	}
	lastT, err2 := time.Parse(time.RFC3339Nano, lastStr)
	if err2 != nil {
		lastT, err2 = time.Parse(time.RFC3339, lastStr)
	}
	if err1 != nil || err2 != nil {
		return float64(constants.DefaultInitValue)
	}
	dur := lastT.Sub(firstT)
	if dur < Config.VelocityMinHistory {
		return float64(constants.DefaultInitValue)
	}
	days := dur.Hours() / HoursPerDay
	if days <= float64(constants.DefaultInitValue) {
		return float64(constants.DefaultInitValue)
	}
	v := float64(totalChanges) / days
	return math.Round(v*VelocityRoundFactor) / VelocityRoundFactor
}
