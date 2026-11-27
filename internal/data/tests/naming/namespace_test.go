package naming

import (
	"testing"

	globalshared "github.com/telark/telark/internal/data/shared"
)

func TestReleaseNamespace(t *testing.T) {
	cases := map[string]struct {
		podNamespace string
		want         string
	}{
		"the pod's namespace":             {podNamespace: "ops", want: "ops"},
		"telark when the chart sets none": {podNamespace: "", want: globalshared.DefaultNamespace},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv(globalshared.EnvPodNamespace, c.podNamespace)
			if got := globalshared.ReleaseNamespace(); got != c.want {
				t.Errorf("namespace = %q, want %q", got, c.want)
			}
		})
	}
}
