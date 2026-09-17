package derivation

import "testing"

func TestReferencedConfigMapsAndSecretsJoinTheirWorkloadsApp(t *testing.T) {
	in := []ResourceInput{
		{Namespace: "n", Kind: "Deployment", Name: "web", Labels: map[string]string{"app": "web"}, ConfigMapRefs: []string{"web-cfg"}, SecretRefs: []string{"web-creds"}},
		{Namespace: "n", Kind: "ConfigMap", Name: "web-cfg"},
		{Namespace: "n", Kind: "Secret", Name: "web-creds"},
		{Namespace: "n", Kind: "ConfigMap", Name: "unrelated"},
		{Namespace: "n", Kind: "Secret", Name: "sh.helm.release.v1.web.v3"},
	}
	groups := map[string]string{}
	for _, r := range GroupByWorkloadAnchor(in) {
		groups[r.Kind+"/"+r.Name] = r.Group
	}
	if groups["ConfigMap/web-cfg"] != "web" || groups["Secret/web-creds"] != "web" {
		t.Fatalf("referenced config must join app web, got %v", groups)
	}
	if _, ok := groups["ConfigMap/unrelated"]; ok {
		t.Fatalf("unreferenced ConfigMap must stay out, got %v", groups)
	}
	if _, ok := groups["Secret/sh.helm.release.v1.web.v3"]; ok {
		t.Fatalf("helm release secret is noise, got %v", groups)
	}
}
