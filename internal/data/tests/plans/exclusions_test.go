package plans

import (
	"testing"

	"github.com/telark/telark/internal/data/plans"
)

const (
	kindConfigMap   = "ConfigMap"
	kindSecret      = "Secret"
	excludedNS      = "shop"
	excludedConfig  = "settings"
	excludedSecrets = "credentials"
)

func excluding(kinds []string, resources ...plans.ProtectionPlanExcludedResource) *plans.ProtectionPlanScopeExclusions {
	return &plans.ProtectionPlanScopeExclusions{Kinds: kinds, Resources: resources}
}

// Exclusions compare as sets: absent and empty are the same, and neither order nor repeats
// count as a change, so an edit that only reorders them is no change to the plan.
func TestExclusionsEqual(t *testing.T) {
	config := plans.ProtectionPlanExcludedResource{Namespace: excludedNS, Kind: kindConfigMap, Name: excludedConfig}
	secret := plans.ProtectionPlanExcludedResource{Namespace: excludedNS, Kind: kindSecret, Name: excludedSecrets}
	cases := []struct {
		name string
		a, b *plans.ProtectionPlanScopeExclusions
		want bool
	}{
		{"absent and empty", nil, excluding(nil), true},
		{"absent and set", nil, excluding([]string{kindSecret}), false},
		{"reordered and repeated", excluding([]string{kindSecret, kindConfigMap}, secret, config),
			excluding([]string{kindConfigMap, kindSecret, kindSecret}, config, secret, config), true},
		{"another kind", excluding([]string{kindSecret}), excluding([]string{kindConfigMap}), false},
		{"another resource", excluding(nil, config), excluding(nil, secret), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := plans.ExclusionsEqual(c.a, c.b); got != c.want {
				t.Errorf("ExclusionsEqual(a, b) = %v, want %v", got, c.want)
			}
			if got := plans.ExclusionsEqual(c.b, c.a); got != c.want {
				t.Errorf("ExclusionsEqual(b, a) = %v, want %v", got, c.want)
			}
		})
	}
}
