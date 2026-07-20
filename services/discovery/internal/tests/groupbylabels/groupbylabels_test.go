package groupbylabels

import (
	"testing"

	"github.com/telark/discovery/internal/discovery/derivation"
	"github.com/telark/discovery/internal/discovery/groupbylabels"
	"github.com/telark/discovery/internal/tests/testutil"
)

// An empty input yields a well-formed, empty payload with a non-nil slice.
func TestBuildGroupByLabelsDataEmpty(t *testing.T) {
	data := groupbylabels.BuildGroupByLabelsData(nil)
	testutil.Equal(t, "total resources", data.TotalResources, 0)
	testutil.Equal(t, "total applications", data.TotalApplications, 0)
	if data.Applications == nil {
		t.Fatal("applications slice should be non-nil")
	}
}

// Resources grouped under two app names assemble into two applications, and the
// resource total reflects every input row.
func TestBuildGroupByLabelsDataGroups(t *testing.T) {
	withGroups := []derivation.ResourceWithGroup{
		{Group: "web", Namespace: "prod", Kind: "Deployment", Name: "web", Images: []string{"nginx:1"}},
		{Group: "web", Namespace: "prod", Kind: "Service", Name: "web-svc"},
		{Group: "api", Namespace: "prod", Kind: "Deployment", Name: "api"},
	}
	data := groupbylabels.BuildGroupByLabelsData(withGroups)
	testutil.Equal(t, "total resources", data.TotalResources, 3)
	testutil.Equal(t, "total applications", data.TotalApplications, 2)
	testutil.Equal(t, "applications len", len(data.Applications), 2)
}
