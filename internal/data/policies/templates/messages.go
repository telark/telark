package templates

import (
	"fmt"
	"strings"

	"github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/data/plans"
	"github.com/telark/telark/internal/data/policies"
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
	// Names the rule's real reach: a volumeMount carries no reference to what backs it, so every
	// volumeMount is compared.
	msgBlockConfigMountChng = "Workload volume mount or config source changes are blocked by protection plan %q."

	enforceSingular = " is blocked by "
	enforcePlural   = " are blocked by "
	auditWording    = " would be blocked by "
)

// An audit plan admits the request, so its message must not claim it was blocked.
var auditReplacer = strings.NewReplacer(enforceSingular, auditWording, enforcePlural, auditWording)

// Kyverno prints a CEL policy's message as written, so the name needs no escaping; %q keeps it on
// one line, as the CRD requires. The audit wording goes into the format, never into the name.
func blockMessage(meta policies.RenderMeta, format string) string {
	if meta.Mode == plans.ModeAudit {
		format = auditReplacer.Replace(format)
	}
	name := meta.PlanName
	if name == constants.EmptyString {
		name = meta.PlanID
	}
	return fmt.Sprintf(format, name)
}
