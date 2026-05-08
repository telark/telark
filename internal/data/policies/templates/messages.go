package templates

const (
	msgBlockCreate          = "Resource creation is blocked by protection plan %q."
	msgBlockUpdate          = "Resource updates are blocked by protection plan %q."
	msgBlockDelete          = "Resource deletion is blocked by protection plan %q."
	msgBlockImagePatterns   = "Container image is blocked by protection plan %q."
	msgBlockImageTags       = "Container image tag is blocked by protection plan %q."
	msgBlockReplicaScaling  = "Replica scaling is blocked by protection plan %q."
	msgBlockPVCMutation     = "PersistentVolumeClaim changes are blocked by protection plan %q."
	msgBlockVolumeChanges   = "Workload volume changes are blocked by protection plan %q."
	msgBlockConfigSecret    = "ConfigMap and Secret changes are blocked by protection plan %q."
	msgBlockConfigMountChng = "Workload ConfigMap or Secret mount changes are blocked by protection plan %q."
)
