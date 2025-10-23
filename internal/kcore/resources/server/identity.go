package server

import (
	"strings"

	"github.com/plsyro/kcore/constants"
	coreclient "github.com/plsyro/kcore/resources/core"
	k8scorev1 "k8s.io/api/core/v1"
)

func DetectClusterMeta() (name, provider, distribution, region string) {
	// Only provider and region are inferred as name and distribution left empty intentionally
	nodes, _ := coreclient.GetNodes()
	if len(nodes) > 0 {
		provider = detectProviderFromNodes(nodes)
		region = detectRegionFromNodes(nodes)
	}
	return "", provider, "", region
}

func detectProviderFromNodes(nodes []k8scorev1.Node) string {
	counts := map[string]int{}
	hasEmpty := false
	for _, n := range nodes {
		pid := strings.ToLower(n.Spec.ProviderID)
		if pid == "" {
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
		}
		counts[p]++
	}
	if len(counts) == 0 && hasEmpty {
		return "baremetal"
	}
	max := 0
	top := ""
	for p, c := range counts {
		if c > max {
			max = c
			top = p
		}
	}
	return top
}

func detectRegionFromNodes(nodes []k8scorev1.Node) string {
	for _, n := range nodes {
		if r, ok := n.Labels[constants.TopologyRegionLabel]; ok && r != "" {
			return r
		}
	}
	return ""
}
