package helpersshared

import (
	"testing"
	"time"

	"github.com/telark/discovery/internal/core/applications/history/utils"
	"github.com/telark/discovery/internal/tests/testutil"
)

const strconvSample = 42

var sampleTime = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// The small pure helpers behave as advertised: zero times format empty, keys and
// pointers round-trip, and slices index into set maps.
func TestUtilsHelpers(t *testing.T) {
	testutil.Equal(t, "zero time", utils.FormatAppTime(time.Time{}), "")
	if utils.FormatAppTime(sampleTime) == "" {
		t.Fatal("non-zero time formatted empty")
	}
	testutil.Equal(t, "resource key", utils.ResourceKey("Deployment", "web"), "Deployment/web")
	testutil.Equal(t, "strptr", *utils.StrPtr("x"), "x")
	testutil.Equal(t, "strconv", utils.StrconvInt(strconvSample), "42")

	m := utils.SliceToMap([]string{"a", "b"})
	testutil.Equal(t, "slice map", m["a"], true)
	is := utils.IntSliceToSet([]int{7})
	testutil.Equal(t, "int set", is[7], true)
}

// ParseRFC3339OrNano accepts both layouts and returns the zero time for blank or
// unparseable input.
func TestParseRFC3339OrNano(t *testing.T) {
	if utils.ParseRFC3339OrNano("2026-01-02T03:04:05Z").IsZero() {
		t.Fatal("valid RFC3339 parsed as zero")
	}
	if utils.ParseRFC3339OrNano("2026-01-02T03:04:05.123456789Z").IsZero() {
		t.Fatal("valid RFC3339Nano parsed as zero")
	}
	testutil.Equal(t, "blank", utils.ParseRFC3339OrNano("  ").IsZero(), true)
	testutil.Equal(t, "garbage", utils.ParseRFC3339OrNano("not-a-time").IsZero(), true)
}
