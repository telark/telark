package derivation

import (
	"testing"

	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/discovery/derivation"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	keyedApp        = "orders"
	keyedWorkload   = "orders-api"
	keyedNamespace  = "sales"
	otherApp        = "suite"
	labelName       = "app.kubernetes.io/name"
	labelPartOf     = "app.kubernetes.io/part-of"
	labelComponent  = "app.kubernetes.io/component"
	labelInstance   = "app.kubernetes.io/instance"
	labelLegacyApp  = "app"
	componentValue  = "api"
	releaseInstance = keyedApp + "-release"
)

// Informers find an app's namespaces by AppKey, so it must name the group
// GroupByWorkloadAnchor puts a labeled workload in, for every identity signal.
func TestAppKeyNamesTheGroup(t *testing.T) {
	cases := []struct {
		name   string
		labels map[string]string
	}{
		{name: "name label", labels: map[string]string{labelName: keyedApp, labelPartOf: otherApp}},
		{name: "part-of label", labels: map[string]string{labelPartOf: keyedApp, labelLegacyApp: otherApp}},
		{name: "component and release instance", labels: map[string]string{labelComponent: componentValue, labelInstance: releaseInstance}},
		{name: "legacy app label", labels: map[string]string{labelLegacyApp: keyedApp}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			groups := derivation.GroupByWorkloadAnchor([]derivation.ResourceInput{
				{Namespace: keyedNamespace, Kind: "Deployment", Name: keyedWorkload, Labels: tc.labels},
			})
			testutil.Equal(t, "groups", len(groups), constants.DefaultAddValue)
			testutil.Equal(t, "group", groups[0].Group, keyedApp)
			testutil.Equal(t, "app key", derivation.AppKey(tc.labels), keyedApp)
		})
	}
	testutil.Equal(t, "unlabeled", derivation.AppKey(nil), constants.EmptyString)
}

// A label value carries case and underscores a CR name cannot; the key names the
// CR, so it is lowercased with underscores as dashes for every identity signal.
func TestAppKeyIsNamedLikeACR(t *testing.T) {
	const raw = "e2e-a-Special_Name.v2"
	const key = "e2e-a-special-name.v2"
	for name, labels := range map[string]map[string]string{
		"name label":     {labelName: raw},
		"part-of label":  {labelPartOf: raw},
		"instance label": {labelComponent: componentValue, labelInstance: raw + "-release"},
		"legacy label":   {labelLegacyApp: raw},
	} {
		testutil.Equal(t, name, derivation.AppKey(labels), key)
	}
}
