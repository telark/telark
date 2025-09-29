package server

import (
	"context"
	"strings"

	"github.com/plsyro/kcore/k8sclient"
	coreclient "github.com/plsyro/kcore/resources/core"
	k8scorev1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmd "k8s.io/client-go/tools/clientcmd"
)

// DetectClusterMeta returns cluster name, provider, distribution and region using dynamic signals.
func DetectClusterMeta() (name, provider, distribution, region string) {
	// Nodes for provider and region
	nodes, _ := coreclient.GetNodes()
	if len(nodes) > 0 {
		provider = detectProviderFromNodes(nodes)
		region = detectRegionFromNodes(nodes)
	}

	// Distribution from discovery (e.g., OpenShift API groups)
	distribution = detectDistribution()

	// Cluster name from kube-public/cluster-info ConfigMap kubeconfig
	if n := getClusterNameFromClusterInfo(); n != "" {
		name = n
	}

	return
}

func detectProviderFromNodes(nodes []k8scorev1.Node) string {
	counts := map[string]int{}
	for _, n := range nodes {
		pid := n.Spec.ProviderID
		if pid == "" {
			continue
		}
		provider := pid
		if idx := strings.Index(pid, "://"); idx > 0 {
			provider = pid[:idx]
		}
		provider = strings.ToLower(provider)
		counts[provider]++
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
		if r, ok := n.Labels["topology.kubernetes.io/region"]; ok && r != "" {
			return r
		}
	}
	return ""
}

func detectDistribution() string {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return ""
	}
	groups, err := client.Discovery().ServerGroups()
	if err != nil || groups == nil {
		return ""
	}
	for _, g := range groups.Groups {
		if strings.Contains(g.Name, "openshift.io") {
			return "openshift"
		}
	}
	return ""
}

func getClusterNameFromClusterInfo() string {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return ""
	}
	cm, err := client.CoreV1().ConfigMaps("kube-public").Get(
		context.Background(),
		"cluster-info",
		k8smetav1.GetOptions{},
	)
	if err != nil || cm == nil {
		return ""
	}
	kubeconfig, ok := cm.Data["kubeconfig"]
	if !ok || kubeconfig == "" {
		return ""
	}
	cfg, err := clientcmd.Load([]byte(kubeconfig))
	if err != nil || cfg == nil {
		return ""
	}
	// Prefer current-context cluster name
	if ctx, ok := cfg.Contexts[cfg.CurrentContext]; ok && ctx != nil {
		if ctx.Cluster != "" {
			return ctx.Cluster
		}
	}
	// Fallback to first cluster key
	for name := range cfg.Clusters {
		return name
	}
	return ""
}
