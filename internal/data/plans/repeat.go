package plans

import (
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/telark/telark/internal/data/constants"
	globalshared "github.com/telark/telark/internal/data/shared"
)

const (
	repeatClockLayout = "15:04"
	repeatWallLayout  = time.DateOnly + " " + repeatClockLayout
	repeatLocalZone   = "Local"
	repeatDayNameLen  = 3
	minutesPerHour    = 60
	// The window that holds now can start the previous local day. The next one starts within a week,
	// or two when a daylight-saving gap leaves that week's occurrence with no length.
	repeatScanFirstDay = -1
	repeatScanLastDay  = 14
	repeatOffsetProbe  = 24 * time.Hour
)

// Week order, the order days are stored in.
var RepeatDays = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

var (
	ErrRepeatRequired = errors.New("repeat is required when timeMode is repeating")
	ErrRepeatDays     = errors.New("repeat.days must list 1 to 7 different days from mon, tue, wed, thu, fri, sat, sun")
	ErrRepeatTime     = errors.New("repeat.startTime and repeat.endTime must be HH:MM (24-hour)")
	ErrRepeatSameTime = errors.New("repeat.endTime must differ from repeat.startTime")
	ErrRepeatTimeZone = errors.New("repeat.timeZone must be an IANA time zone name, for example Europe/Paris")
)

func ValidateRepeat(r *ProtectionPlanRepeat) error {
	_, _, _, err := parseRepeat(r)
	return err
}

// The window that holds now, else the next one. A window starts on each listed local weekday,
// and one whose end is earlier than its start ends the next day.
func windowAt(r *ProtectionPlanRepeat, now time.Time) (start, end time.Time, ok bool) {
	loc, startMinutes, endMinutes, err := parseRepeat(r)
	if err != nil {
		return start, end, false
	}
	today, _ := time.Parse(time.DateOnly, now.In(loc).Format(time.DateOnly))
	for offset := repeatScanFirstDay; offset <= repeatScanLastDay; offset++ {
		day := today.AddDate(constants.DefaultInitValue, constants.DefaultInitValue, offset)
		if !slices.Contains(r.Days, strings.ToLower(day.Weekday().String()[:repeatDayNameLen])) {
			continue
		}
		endDay := day
		if endMinutes < startMinutes {
			endDay = day.AddDate(constants.DefaultInitValue, constants.DefaultInitValue, constants.SingleItem)
		}
		occurrenceStart, occurrenceEnd := wallClock(day, startMinutes, loc), wallClock(endDay, endMinutes, loc)
		// A gap can leave an occurrence with no length (Paris 02:00 to 03:00 on the spring-forward day).
		if !occurrenceEnd.After(occurrenceStart) {
			continue
		}
		// A daylight-saving gap can push an end past the next start: the two run as one window.
		if ok && !occurrenceStart.After(end) {
			if occurrenceEnd.After(end) {
				end = occurrenceEnd
			}
			continue
		}
		if ok && end.After(now) {
			return start, end, true
		}
		start, end, ok = occurrenceStart, occurrenceEnd, true
	}
	return start, end, ok && end.After(now)
}

// The window a repeating plan is in or waits for, in the form status.window stores, and whether
// now is inside it. Other plans have none.
func PlanWindow(plan *ProtectionPlan, now time.Time) (*ProtectionPlanTimeRange, bool) {
	if plan.TimeMode != TimeModeRepeating {
		return nil, false
	}
	start, end, ok := windowAt(plan.Repeat, now)
	if !ok {
		return nil, false
	}
	return &ProtectionPlanTimeRange{
		StartAt: start.UTC().Format(globalshared.DefaultTimeFormat),
		EndAt:   end.UTC().Format(globalshared.DefaultTimeFormat),
	}, !start.After(now)
}

func parseRepeat(r *ProtectionPlanRepeat) (loc *time.Location, startMinutes, endMinutes int, err error) {
	if r == nil {
		return nil, startMinutes, endMinutes, ErrRepeatRequired
	}
	if !validDays(r.Days) {
		return nil, startMinutes, endMinutes, ErrRepeatDays
	}
	startMinutes, startErr := clockMinutes(r.StartTime)
	endMinutes, endErr := clockMinutes(r.EndTime)
	if startErr != nil || endErr != nil {
		return nil, startMinutes, endMinutes, ErrRepeatTime
	}
	if startMinutes == endMinutes {
		return nil, startMinutes, endMinutes, ErrRepeatSameTime
	}
	// Both load without an error and neither is a named zone: "" is UTC and "Local" the process's own.
	if r.TimeZone == constants.EmptyString || r.TimeZone == repeatLocalZone {
		return nil, startMinutes, endMinutes, ErrRepeatTimeZone
	}
	if loc, err = time.LoadLocation(r.TimeZone); err != nil {
		return nil, startMinutes, endMinutes, ErrRepeatTimeZone
	}
	return loc, startMinutes, endMinutes, nil
}

func validDays(days []string) bool {
	if len(days) == constants.DefaultInitValue || len(days) > len(RepeatDays) {
		return false
	}
	for i, day := range days {
		if !slices.Contains(RepeatDays, day) || slices.Contains(days[:i], day) {
			return false
		}
	}
	return true
}

// time.Parse reads "2:00" with "15:04", so the length pins the two-digit form the CRD requires.
func clockMinutes(value string) (int, error) {
	parsed, err := time.Parse(repeatClockLayout, value)
	if err == nil && len(value) != len(repeatClockLayout) {
		err = ErrRepeatTime
	}
	return parsed.Hour()*minutesPerHour + parsed.Minute(), err
}

// RFC 5545 §3.3.5: a wall time a daylight-saving gap skips takes the offset in force before the
// gap, and a wall time that occurs twice is its first occurrence.
func wallClock(day time.Time, minutes int, loc *time.Location) time.Time {
	wall := day.Add(time.Duration(minutes) * time.Minute)
	before := zoneOffset(wall.Add(-repeatOffsetProbe), loc)
	resolved, found := wall.Add(-before), false
	for _, offset := range []time.Duration{before, zoneOffset(wall.Add(repeatOffsetProbe), loc)} {
		candidate := wall.Add(-offset)
		if candidate.In(loc).Format(repeatWallLayout) != wall.Format(repeatWallLayout) {
			continue
		}
		if !found || candidate.Before(resolved) {
			resolved, found = candidate, true
		}
	}
	return resolved
}

func zoneOffset(at time.Time, loc *time.Location) time.Duration {
	_, seconds := at.In(loc).Zone()
	return time.Duration(seconds) * time.Second
}
