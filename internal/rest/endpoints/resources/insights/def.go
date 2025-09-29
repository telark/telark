package insights

import "github.com/plsyro/rest/base"

// ClusterInsights does not define a name as they are unique per cluster
const (
	CreateClusterInsight base.Endpoint = "resources/insights/cluster/create"
	GetClusterInsight    base.Endpoint = "resources/insights/cluster/get"
)
