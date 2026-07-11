package analyze

import "github.com/telark/rest/base"

const (
	GetAllWorkloadsByNamespace base.Endpoint = "analyze/workloads/{namespace}/get"
	GetAllResourcesByNamespace base.Endpoint = "analyze/resources/{namespace}/get"
	GetAllNamespaces           base.Endpoint = "analyze/namespaces/get"
)
