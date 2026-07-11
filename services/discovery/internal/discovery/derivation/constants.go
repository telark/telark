package derivation

import kcoreshared "github.com/telark/kcore/shared"

const (
	// K8s common labels (https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/)
	labelAppName   = "app.kubernetes.io/name"
	labelPartOf    = "app.kubernetes.io/part-of"
	labelComponent = "app.kubernetes.io/component"
	labelInstance  = "app.kubernetes.io/instance"
	labelAppLegacy = "app"
	releaseSuffix  = "-release"

	// noise resources
	noiseResourceKubeRootCaCrt           = "kube-root-ca.crt"
	noiseResourceDefault                 = "default"
	noiseResourceKubernetes              = "kubernetes"
	noiseResourceSecretHelmReleasePrefix = "sh.helm.release."

	// common constants
	repeatedRatio           = 0.3
	minRepeatedNum          = 2
	fallbackGroup           = "Other"
	resourceMapKeySeparator = "\x00"

	// Signal names for identity resolution (used in debug logging).
	signalAppName      = labelAppName
	signalPartOf       = "app.kubernetes.io/part-of"
	signalComponent    = "component+instance"
	signalAppLegacy    = "app"
	signalUnidentified = "UNIDENTIFIED"

)

var workloadKinds = func() map[string]bool {
	out := make(map[string]bool)
	for _, gvr := range kcoreshared.AppGVRs() {
		if gvr.Group == "apps" || gvr.Group == "batch" {
			out[kcoreshared.ResourceKind(gvr.Resource)] = true
		}
	}
	return out
}()

var GroupNameLabelKeys = []string{labelAppName, labelAppLegacy}
