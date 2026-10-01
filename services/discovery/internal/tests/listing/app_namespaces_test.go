package listing

import (
	"context"
	"slices"
	"testing"

	"github.com/telark/telark/services/discovery/internal/discovery/listing"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	appName      = "rl-skew"
	jobNamespace = "recs-lab"
)

// Without the informer cache (before it is wired) the caller's namespaces stand.
func TestAppNamespacesWithoutCache(t *testing.T) {
	got := listing.AppNamespaces(context.Background(), appName, []string{jobNamespace})
	testutil.Equal(t, "namespaces", slices.Equal(got, []string{jobNamespace}), true)
}
