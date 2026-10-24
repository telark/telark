package templates

import (
	"fmt"

	"github.com/telark/telark/internal/data/policies"
)

const (
	opCreate = "CREATE"
	opUpdate = "UPDATE"
	opDelete = "DELETE"

	kindDeployment       = "Deployment"
	kindDeploymentScale  = "Deployment/scale"
	kindStatefulSet      = "StatefulSet"
	kindStatefulSetScale = "StatefulSet/scale"
	kindPVC              = "PersistentVolumeClaim"
	kindConfigMap        = "ConfigMap"
	kindSecret           = "Secret"

	admitNever = "false"
	// A scale request's object is the Scale, which carries spec.replicas the same way.
	admitSameReplicas  = `object.?spec.?replicas.orValue(0) == oldObject.?spec.?replicas.orValue(0)`
	admitNoImageFmt    = `!%s.exists(c, c.?image.orValue("").matches(%s))`
	exprImagesFmt      = `%s.map(c, c.?image.orValue(""))`
	admitNoImageTagFmt = `!%s.exists(i, isImage(i) && parseImageReference(i).tag() in %s)`
	admitSameFmt       = "%s == %s"
	celAnd             = " && "
	celOr              = " || "

	// %s is a pod spec. Whole volume sources, not just the name: items, defaultMode and optional
	// decide which keys land in the container.
	exprVolumesFmt       = `%s.?volumes.orValue([])`
	exprCMVolumesFmt     = `%s.?volumes.orValue([]).filter(v, has(v.configMap)).map(v, v.configMap)`
	exprSecretVolumesFmt = `%s.?volumes.orValue([]).filter(v, has(v.secret)).map(v, v.secret)`
	// %s is every container of the pod spec, init and ephemeral included.
	exprEnvFromFmt = `%s.map(c, c.?envFrom.orValue([])).flatten()`
	exprEnvRefsFmt = `%s.map(c, c.?env.orValue([])).flatten()` +
		`.filter(e, has(e.valueFrom) && (has(e.valueFrom.configMapKeyRef) || has(e.valueFrom.secretKeyRef)))` +
		`.map(e, e.valueFrom)`
	exprVolumeMountsFmt = `%s.map(c, c.?volumeMounts.orValue([])).flatten()`
)

// Holds when the projection reads the same on the new and the old pod spec (or container list).
func same(format, newRoot, oldRoot string) string {
	return fmt.Sprintf(admitSameFmt, fmt.Sprintf(format, newRoot), fmt.Sprintf(format, oldRoot))
}

var (
	opsCreate             = []string{opCreate}
	opsUpdate             = []string{opUpdate}
	opsDelete             = []string{opDelete}
	opsCreateUpdate       = []string{opCreate, opUpdate}
	opsUpdateDelete       = []string{opUpdate, opDelete}
	opsCreateUpdateDelete = []string{opCreate, opUpdate, opDelete}
	kindsWildcard         = []string{policies.KindWildcard}
	kindsReplicaTarget    = []string{kindDeployment, kindDeploymentScale, kindStatefulSet, kindStatefulSetScale}
	kindsPVC              = []string{kindPVC}
	kindsConfigSecret     = []string{kindConfigMap, kindSecret}
)
