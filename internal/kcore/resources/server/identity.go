package server

import (
	"strings"

	"github.com/telark/kcore/constants"
	coreclient "github.com/telark/kcore/resources/core"
	k8scorev1 "k8s.io/api/core/v1"
)

func DetectClusterMeta() (name, provider, distribution, region string) {
	// Only provider and region are inferred as name and distribution left empty intentionally
	nodes, _ := coreclient.GetNodes()
	if len(nodes) > constants.ZeroValue {
		provider = detectProviderFromNodes(nodes)
		region = detectRegionFromNodes(nodes)
	}
	return constants.EmptyString, provider, constants.EmptyString, region
}

func detectProviderFromNodes(nodes []k8scorev1.Node) string {
	counts := map[string]int{}
	hasEmpty := false
	for _, n := range nodes {
		pid := strings.ToLower(n.Spec.ProviderID)
		if pid == constants.EmptyString {
			hasEmpty = true
			continue
		}
		p := pid
		if before, _, found := strings.Cut(pid, "://"); found && before != "" {
			p = before
		}
		switch p {
		case "gce":
			p = "gcp"
		case "k3s":
			p = "rancher labs"
		default:
		}
		counts[p]++
	}
	if len(counts) == constants.ZeroValue && hasEmpty {
		return "baremetal"
	}
	maxCount := constants.ZeroValue
	top := constants.EmptyString
	for p, c := range counts {
		if c > maxCount {
			maxCount = c
			top = p
		}
	}
	return top
}

func detectRegionFromNodes(nodes []k8scorev1.Node) string {
	for _, n := range nodes {
		if r, ok := n.Labels[constants.TopologyRegionLabel]; ok && r != constants.EmptyString {
			return r
		}
	}
	return constants.EmptyString
}
