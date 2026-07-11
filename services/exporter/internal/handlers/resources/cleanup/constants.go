package cleanup

import "time"

const (
	fieldSpec                    = "spec"
	fieldMetadata                = "metadata"
	fieldFinalizers              = "finalizers"
	messageNoChange              = "finalizers unchanged"
	messageFinalizersUpdated     = "finalizers updated"
	messageUnknownResourceType   = "unknown resource type"
	messageFinalizerNameRequired = "finalizer name is required"
	messageViewProjected         = "cleanup view projected"
	messageViewsProjected        = "cleanup views projected"
	timeFormatRFC3339            = time.RFC3339
)
