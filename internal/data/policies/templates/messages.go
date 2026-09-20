package templates

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
	// The rule compares every volumeMount, not only those backed by a ConfigMap or Secret: a
	// volumeMount carries no reference to what backs it and JMESPath cannot join it to the
	// volume list, so the message names the reach the rule actually has.
	msgBlockConfigMountChng = "Workload volume mount or config source changes are blocked by protection plan %q."
)
