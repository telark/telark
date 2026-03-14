package analyze

import "github.com/plsyro/rest/base"

const (
	Startanalyze               base.Endpoint = "analyze/start"
	GetAllWorkloadsByNamespace base.Endpoint = "analyze/workloads/{namespace}/get"
	GetAllResourcesByNamespace base.Endpoint = "analyze/resources/{namespace}/get"
)
