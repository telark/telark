package planids

import (
	"regexp"
	"testing"

	"github.com/telark/discovery/internal/core/plans/protection/ids"
	"github.com/telark/discovery/internal/tests/testutil"
)

var planIDPattern = regexp.MustCompile(`^pp-[a-z0-9]{3}-[a-z0-9]{4}-[a-z0-9]{4}$`)

// A generated plan ID matches the CRD's required pp-<3>-<4>-<4> shape, and
// successive calls differ.
func TestGeneratePlanID(t *testing.T) {
	first, err := ids.GeneratePlanID()
	testutil.Equal(t, "err", err, nil)
	if !planIDPattern.MatchString(first) {
		t.Fatalf("plan id %q does not match %s", first, planIDPattern)
	}
	second, err := ids.GeneratePlanID()
	testutil.Equal(t, "err2", err, nil)
	if first == second {
		t.Fatalf("two generated ids collided: %q", first)
	}
}
