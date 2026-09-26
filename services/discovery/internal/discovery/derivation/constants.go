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
	// A label value may carry what a CR name cannot; the key is what the CR is named after.
	appKeyUnderscore = "_"
	appKeyDash       = "-"

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
	signalPartOf       = labelPartOf
	signalComponent    = "component+instance"
	signalAppLegacy    = labelAppLegacy
	signalUnidentified = "UNIDENTIFIED"

	// Job runs spawned by a CronJob come and go every schedule tick; the
	// CronJob represents them, so they are not application resources.
	kindJob       = "Job"
	kindCronJob   = "CronJob"
	kindConfigMap = "ConfigMap"
	kindSecret    = "Secret"
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
