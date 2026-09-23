package derivation

import (
	"testing"

	"github.com/telark/discovery/internal/discovery/derivation"
)

const (
	inputNamespace = "n"
	webApp         = "web"
)

func TestReferencedConfigMapsAndSecretsJoinTheirWorkloadsApp(t *testing.T) {
	in := []derivation.ResourceInput{
		{
			Namespace: inputNamespace, Kind: "Deployment", Name: webApp, Labels: map[string]string{"app": webApp},
			ConfigMapRefs: []string{"web-cfg"}, SecretRefs: []string{"web-creds"},
		},
		{Namespace: inputNamespace, Kind: "ConfigMap", Name: "web-cfg"},
		{Namespace: inputNamespace, Kind: "Secret", Name: "web-creds"},
		{Namespace: inputNamespace, Kind: "ConfigMap", Name: "unrelated"},
		{Namespace: inputNamespace, Kind: "Secret", Name: "sh.helm.release.v1.web.v3"},
	}
	groups := map[string]string{}
	for _, r := range derivation.GroupByWorkloadAnchor(in) {
		groups[r.Kind+"/"+r.Name] = r.Group
	}
	if groups["ConfigMap/web-cfg"] != webApp || groups["Secret/web-creds"] != webApp {
		t.Fatalf("referenced config must join app web, got %v", groups)
	}
	if _, ok := groups["ConfigMap/unrelated"]; ok {
		t.Fatalf("unreferenced ConfigMap must stay out, got %v", groups)
	}
	if _, ok := groups["Secret/sh.helm.release.v1.web.v3"]; ok {
		t.Fatalf("helm release secret is noise, got %v", groups)
	}
}
