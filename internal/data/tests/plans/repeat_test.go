package plans

import (
	"errors"
	"testing"
	"time"

	"github.com/telark/telark/internal/data/plans"
)

const (
	zoneParis    = "Europe/Paris"
	zoneNewYork  = "America/New_York"
	zoneLordHowe = "Australia/Lord_Howe"
	zoneTokyo    = "Asia/Tokyo"
	zoneUnknown  = "Mars/Olympus"

	daySun      = "sun"
	dayMon      = "mon"
	dayFri      = "fri"
	twoAM       = "02:00"
	halfPastTwo = "02:30"
	threeAM     = "03:00"
	fourAM      = "04:00"
	// Sunday 2026-10-18 02:00 to 04:00 in Tokyo.
	tokyoSundayStart  = "2026-10-17T17:00:00Z"
	tokyoSundayEnd    = "2026-10-17T19:00:00Z"
	tokyoSundayEarly  = "2026-10-17T16:00:00Z"
	tokyoSundayInside = "2026-10-17T18:00:00Z"
	// Saturday noon before Paris springs forward on Sunday 2026-03-29.
	parisSpringSaturday = "2026-03-28T12:00:00Z"
)

func rule(zone, startTime, endTime string, days ...string) *plans.ProtectionPlanRepeat {
	return &plans.ProtectionPlanRepeat{Days: days, StartTime: startTime, EndTime: endTime, TimeZone: zone}
}

func at(t *testing.T, raw string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

// Every case reads the window through PlanWindow, so the daylight-saving resolver and the scan are
// checked together, in zones whose rules differ (Paris and New York disagree in Go's time.Date).
func TestPlanWindowFindsTheWindow(t *testing.T) {
	everyDay := []string{dayMon, "tue", "wed", "thu", dayFri, "sat", daySun}
	cases := []struct {
		name       string
		repeat     *plans.ProtectionPlanRepeat
		now        string
		start, end string
	}{
		{"before the window", rule(zoneTokyo, twoAM, fourAM, daySun), tokyoSundayEarly,
			tokyoSundayStart, tokyoSundayEnd},
		{"at its start", rule(zoneTokyo, twoAM, fourAM, daySun), tokyoSundayStart,
			tokyoSundayStart, tokyoSundayEnd},
		{"inside", rule(zoneTokyo, twoAM, fourAM, daySun), tokyoSundayInside,
			tokyoSundayStart, tokyoSundayEnd},
		{"at its end the next one starts", rule(zoneTokyo, twoAM, fourAM, daySun), tokyoSundayEnd,
			"2026-10-24T17:00:00Z", "2026-10-24T19:00:00Z"},
		{"one weekday, a week away", rule(zoneTokyo, "10:00", "11:00", dayFri), "2026-10-16T03:00:00Z",
			"2026-10-23T01:00:00Z", "2026-10-23T02:00:00Z"},
		{"the next listed day", rule(zoneTokyo, "09:00", "17:00", dayMon, "wed"), "2026-10-20T03:00:00Z",
			"2026-10-21T00:00:00Z", "2026-10-21T08:00:00Z"},
		{"past midnight, seen after midnight", rule(zoneTokyo, "22:00", twoAM, "sat"), tokyoSundayEarly,
			"2026-10-17T13:00:00Z", tokyoSundayStart},
		{"Paris gap takes the offset before it", rule(zoneParis, halfPastTwo, "05:00", daySun), "2026-03-29T00:00:00Z",
			"2026-03-29T01:30:00Z", "2026-03-29T03:00:00Z"},
		{"New York gap takes the offset before it", rule(zoneNewYork, halfPastTwo, fourAM, daySun), "2026-03-08T05:00:00Z",
			"2026-03-08T07:30:00Z", "2026-03-08T08:00:00Z"},
		{"Lord Howe half-hour gap", rule(zoneLordHowe, "02:15", threeAM, daySun), "2026-10-03T12:00:00Z",
			"2026-10-03T15:45:00Z", "2026-10-03T16:00:00Z"},
		{"Paris repeated time is its first occurrence", rule(zoneParis, halfPastTwo, fourAM, daySun), "2026-10-24T22:00:00Z",
			"2026-10-25T00:30:00Z", "2026-10-25T03:00:00Z"},
		{"New York repeated time is its first occurrence", rule(zoneNewYork, "01:30", threeAM, daySun), "2026-11-01T04:00:00Z",
			"2026-11-01T05:30:00Z", "2026-11-01T08:00:00Z"},
		{"spring forward shortens the window to 2 h", rule(zoneParis, "01:00", fourAM, daySun), parisSpringSaturday,
			"2026-03-29T00:00:00Z", "2026-03-29T02:00:00Z"},
		{"fall back lengthens it to 4 h", rule(zoneParis, "01:00", fourAM, daySun), "2026-10-24T12:00:00Z",
			"2026-10-24T23:00:00Z", "2026-10-25T03:00:00Z"},
		{"an end pushed past the next start merges both", rule(zoneParis, "03:10", "02:40", everyDay...), parisSpringSaturday,
			"2026-03-28T02:10:00Z", "2026-03-30T00:40:00Z"},
		// 02:00 to 03:00 lies in the skipped hour, so that Sunday has no window and the next is a week on.
		{"a window the gap empties does not run that day", rule(zoneParis, twoAM, threeAM, daySun), parisSpringSaturday,
			"2026-04-05T00:00:00Z", "2026-04-05T01:00:00Z"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan := plans.ProtectionPlan{TimeMode: plans.TimeModeRepeating, Repeat: c.repeat}
			window, _ := plans.PlanWindow(&plan, at(t, c.now))
			if window == nil {
				t.Fatal("PlanWindow found no window")
			}
			if window.StartAt != c.start || window.EndAt != c.end {
				t.Errorf("window = %s → %s, want %s → %s", window.StartAt, window.EndAt, c.start, c.end)
			}
			if !at(t, window.EndAt).After(at(t, c.now)) {
				t.Errorf("window end %s is not after now %s", window.EndAt, c.now)
			}
		})
	}
}

func TestPlanWindow(t *testing.T) {
	repeating := plans.ProtectionPlan{TimeMode: plans.TimeModeRepeating, Repeat: rule(zoneTokyo, twoAM, fourAM, daySun)}
	window, open := plans.PlanWindow(&repeating, at(t, tokyoSundayInside))
	if window == nil || window.StartAt != tokyoSundayStart || window.EndAt != tokyoSundayEnd || !open {
		t.Errorf("PlanWindow = %+v, open %v; want the open 17:00Z → 19:00Z window", window, open)
	}
	if _, open := plans.PlanWindow(&repeating, at(t, tokyoSundayEarly)); open {
		t.Error("PlanWindow reports a window open before its start")
	}
	for name, repeat := range map[string]*plans.ProtectionPlanRepeat{
		"without a rule":     nil,
		"in an unknown zone": rule(zoneUnknown, twoAM, fourAM, daySun),
	} {
		if window, _ := plans.PlanWindow(&plans.ProtectionPlan{TimeMode: plans.TimeModeRepeating, Repeat: repeat}, time.Now()); window != nil {
			t.Errorf("PlanWindow = %+v for a repeating plan %s, want none", window, name)
		}
	}
	// A rule left behind after a switch to another time mode is ignored.
	switched := repeating
	switched.TimeMode = plans.TimeModePermanent
	if window, _ := plans.PlanWindow(&switched, time.Now()); window != nil {
		t.Errorf("PlanWindow = %+v for a permanent plan, want none", window)
	}
}

func TestValidateRepeat(t *testing.T) {
	cases := []struct {
		name   string
		repeat *plans.ProtectionPlanRepeat
		want   error
	}{
		{"valid", rule(zoneParis, twoAM, fourAM, daySun, dayMon), nil},
		{"valid in UTC", rule("UTC", "22:00", twoAM, dayFri), nil},
		{"missing", nil, plans.ErrRepeatRequired},
		{"no days", rule(zoneParis, twoAM, fourAM), plans.ErrRepeatDays},
		{"unknown day", rule(zoneParis, twoAM, fourAM, "sunday"), plans.ErrRepeatDays},
		{"duplicate day", rule(zoneParis, twoAM, fourAM, daySun, daySun), plans.ErrRepeatDays},
		{"hour 24", rule(zoneParis, "24:00", fourAM, daySun), plans.ErrRepeatTime},
		{"one-digit hour", rule(zoneParis, "2:00", fourAM, daySun), plans.ErrRepeatTime},
		{"minute 60", rule(zoneParis, twoAM, "04:60", daySun), plans.ErrRepeatTime},
		{"empty time", rule(zoneParis, "", fourAM, daySun), plans.ErrRepeatTime},
		{"equal times", rule(zoneParis, twoAM, twoAM, daySun), plans.ErrRepeatSameTime},
		{"empty zone", rule("", twoAM, fourAM, daySun), plans.ErrRepeatTimeZone},
		{"process zone", rule("Local", twoAM, fourAM, daySun), plans.ErrRepeatTimeZone},
		{"unknown zone", rule(zoneUnknown, twoAM, fourAM, daySun), plans.ErrRepeatTimeZone},
		{"path in the zone", rule("../../etc/passwd", twoAM, fourAM, daySun), plans.ErrRepeatTimeZone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := plans.ValidateRepeat(c.repeat); !errors.Is(got, c.want) {
				t.Errorf("ValidateRepeat = %v, want %v", got, c.want)
			}
		})
	}
}
