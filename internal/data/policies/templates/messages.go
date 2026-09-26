package templates

import (
	"fmt"
	"strings"

	"github.com/telark/data/plans"
	"github.com/telark/data/policies"
)

const (
	msgBlockCreate         = "Resource creation is blocked by protection plan %q."
	msgBlockUpdate         = "Resource updates are blocked by protection plan %q."
	msgBlockDelete         = "Resource deletion is blocked by protection plan %q."
	msgBlockImagePatterns  = "Container image is blocked by protection plan %q."
	msgBlockImageTags      = "Container image tag is blocked by protection plan %q."
	msgBlockReplicaScaling = "Replica scaling is blocked by protection plan %q."
	msgBlockPVCMutation    = "PersistentVolumeClaim creation, modification and deletion are blocked by protection plan %q."
	msgBlockVolumeChanges  = "Workload volume changes are blocked by protection plan %q."
	msgBlockConfigSecret   = "ConfigMap and Secret changes are blocked by protection plan %q."
	// Names the rule's real reach: a volumeMount carries no reference to what backs it and
	// JMESPath cannot join it to the volume list, so every volumeMount is compared.
	msgBlockConfigMountChng = "Workload volume mount or config source changes are blocked by protection plan %q."

	enforceSingular = " is blocked by "
	enforcePlural   = " are blocked by "
	auditWording    = " would be blocked by "
)

// An audit plan admits the request, so its message must not claim it was blocked.
var auditReplacer = strings.NewReplacer(enforceSingular, auditWording, enforcePlural, auditWording)

// Kyverno substitutes {{ }} variables in validate.message, so only the generated plan ID
// goes here; the user-chosen name stays in the plan-name annotation, which is not substituted.
func blockMessage(meta policies.RenderMeta, format string) string {
	msg := fmt.Sprintf(format, meta.PlanID)
	if meta.Mode == plans.ModeAudit {
		return auditReplacer.Replace(msg)
	}
	return msg
}
