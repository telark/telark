package planapps

import (
	"slices"
	"testing"

	"github.com/telark/data/policies"
	"github.com/telark/discovery/internal/core/plans/protection/applications"
)

const (
	appMulti  = "multi"
	appSingle = "single"
	appNone   = "none"
	appGhost  = "ghost"
	nsOne     = "ns1"
	nsTwo     = "ns2"
)

func resolvedApps() map[string]policies.ResolvedApp {
	return map[string]policies.ResolvedApp{
		appMulti:  {Namespaces: []string{nsTwo, nsOne}},
		appSingle: {Namespaces: []string{nsOne}},
		appNone:   {},
	}
}

// Every namespace of every listed application, once; a missing id contributes nothing
// instead of failing the read.
func TestNamespacesFlattensAndSkipsMissing(t *testing.T) {
	got := applications.Namespaces(resolvedApps(), []string{appMulti, appSingle, appGhost})
	if !slices.Equal(got, []string{nsOne, nsTwo}) {
		t.Fatalf("Namespaces = %v, want [%s %s]", got, nsOne, nsTwo)
	}
}

// An application whose every namespace is ignored by the policy engine is unprotectable;
// a missing id is reported elsewhere and not repeated here.
func TestUnprotectable(t *testing.T) {
	got := applications.Unprotectable(resolvedApps(), []string{appMulti, appNone, appGhost})
	if !slices.Equal(got, []string{appNone}) {
		t.Fatalf("Unprotectable = %v, want [%s]", got, appNone)
	}
}
