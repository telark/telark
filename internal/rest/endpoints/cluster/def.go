package cluster

import "github.com/telark/telark/internal/rest/base"

const (
	GetAllWorkloadsByNamespace base.Endpoint = "cluster/namespaces/{namespace}/workloads"
	GetAllResourcesByNamespace base.Endpoint = "cluster/namespaces/{namespace}/resources"
	GetAllNamespaces           base.Endpoint = "cluster/namespaces"
)
