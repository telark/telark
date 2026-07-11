package metrics

import "github.com/telark/data/resources/application"

type (
	WorkloadBaselineReader func(namespace, kind, name string) application.MetricsBaseline
	workloadAnchorKey      struct {
		ns, kind, name string
	}
)
