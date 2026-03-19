package server

import (
	"strings"

	"github.com/plsyro/kcore/constants"
	coreclient "github.com/plsyro/kcore/resources/core"
	k8scorev1 "k8s.io/api/core/v1"
)

const (
	emptyValue  = ""
	initialZero = 0
)

//nolint:revive // Keeping existing public API shape for backward compatibility.
func DetectClusterMeta() (name, provider, distribution, region string) {
	// Only provider and region are inferred as name and distribution left empty intentionally
	nodes, _ := coreclient.GetNodes()
	if len(nodes) > initialZero {
		provider = detectProviderFromNodes(nodes)
		region = detectRegionFromNodes(nodes)
	}
	return emptyValue, provider, emptyValue, region
}

func detectProviderFromNodes(nodes []k8scorev1.Node) string {
	counts := map[string]int{}
	hasEmpty := false
	for _, n := range nodes {
		pid := strings.ToLower(n.Spec.ProviderID)
		if pid == emptyValue {
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
	if len(counts) == initialZero && hasEmpty {
		return "baremetal"
	}
	maxCount := initialZero
	top := emptyValue
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
		if r, ok := n.Labels[constants.TopologyRegionLabel]; ok && r != emptyValue {
			return r
		}
	}
	return emptyValue
}
