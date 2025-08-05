package errors

type Error string

const (
	// REST-related Errors
	ErrRestMarshalPayload            Error = "error while marshaling JSON payload: %v"
	ErrRestEmptyRequestBody          Error = "empty request body"
	ErrRestMarshalUnstructuredToJSON Error = "error while marshaling unstructured to " +
		"JSON: %v"
	ErrRestUnmarshalResponseToGeneric Error = "error while unmarshaling JSON Response to " +
		"Generic Response: %v"
	ErrRestUnmarshalResourceToJSON Error = "error while unmarshaling resource to " +
		"JSON: %v"
	ErrRestUnmarshalRequestBodyToJSON Error = "error while unmarshaling request body to " +
		"JSON: %v"
	ErrRestSendRequest        Error = "error while sending request: %v"
	ErrRestReadResponseBody   Error = "error while reading response body: %v"
	ErrRestReadRequestBody    Error = "error while reading request body: %v"
	ErrRestEncodeResponse     Error = "error while encoding response: %v"
	ErrRestMissingRequestBody Error = "missing request body for the '{action}' " +
		"action."
	ErrRestParseRequestBody Error = "error while parsing request body: %v"
	ErrRestDecodeResponse   Error = "error while decoding response: %v"
	ErrRestMissingParam     Error = "the '{param}' parameter is required in " +
		"the URL path."
	ErrRestGenURL  Error = "failed to generate request URL"
	ErrRestSendReq Error = "failed to send {method} request for"

	// K8s-related Errors
	ErrK8sSetClient    Error = "failed to set Kubernetes client: %v"
	ErrK8sCreateConfig Error = "failed to create Kubernetes in-cluster " +
		"config: %v"
	ErrK8sGetService Error = "error getting Service %s in Namespace " +
		"%s: %v"
	ErrK8sEmptyNamespaceOrResourceName Error = "namespace or resourceName cannot be empty"
	ErrK8sEmptyNamespace               Error = "Namespace cannot be empty"
	ErrK8sFetchingServices             Error = "error fetching services in namespace " +
		"%s: %v"
	ErrK8sFetchingDeployments Error = "error fetching deployments in namespace " +
		"%s: %v"
	ErrK8sFetchingPodEvents Error = "error fetching events for pod %s in " +
		"namespace %s: %v"

	// Resource-related Errors
	ErrCreateResource Error = "error while creating resource %s"
	ErrGetResource    Error = "error while fetching resource " +
		"%s: %v"
	ErrListResources  Error = "error while listing resources: %v"
	ErrUpdateResource Error = "error while updating resource " +
		"%s: %v"
	ErrDeleteResource Error = "error while deleting resource " +
		"%s: %v"
	ErrUpdateResourceWithHistory Error = "error while updating resource " +
		"with history."
	ErrUpdateResourceSync Error = "error while updating resource " +
		"sync settings."
	ErrFilterResources        Error = "error while filtering resource: %v"
	ErrCheckResourceExistence Error = "error while checking resource " +
		"existence"
	ErrResourceExists   Error = "resource already exists"
	ErrResourceNotFound Error = "resource not found"
	ErrPatchResource    Error = "error while patching resource " +
		"%s status: %s"
	ErrConvertResource Error = "error while converting resource " +
		"to %s: %v"
	ErrResourceHasNoFields      Error = "%s has no %s"
	ErrRemoveFieldsFromResource Error = "error while removing %s from %s"
	ErrGenerateManagedResource  Error = "error while generating managed " +
		"resource %s: %v"
	ErrUnsupportedResourceType Error = "unsupported resource type %s: %v"
	ErrAddResourceToGrouper    Error = "error while adding resource " +
		"%s to grouper %s: %v"
	ErrDeleteOperationTimedOut Error = "delete operation timed out"
	ErrRemoveFromGrouper       Error = "failed to remove %s from " +
		"grouper %s: %v"
	ErrRemoveFromAttachedResources Error = "failed to remove %s from " +
		"attached resource: %v"
	ErrBothGroupersMustBeProvidedForComparison Error = "both groupers must be provided " +
		"for comparison"
	ErrMaintenanceObjectIsNil Error = "maintenance object is nil"

	// Build-related Errors
	ErrGrouperCannotBeNil           Error = "grouper cannot be nil"
	ErrGrouperCannotBeEmpty         Error = "grouper cannot be empty"
	ErrSourceNameCannotBeEmpty      Error = "source name cannot be empty"
	ErrResourceNameCannotBeEmpty    Error = "resource name cannot be empty"
	ErrUnsupportedAppWorkloadType   Error = "unsupported App Workload Type: %v" //nolint:gosec
	ErrUnsupportedBatchWorkloadType Error = "unsupported Batch Workload Type: %v"

	// Patch-related Errors
	ErrUnknownWorkloadTypeForBridge  Error = "unknown workload type for Bridge %s"
	ErrUnknownResourceTypeForGrouper Error = "unknown resource type for Grouper %s"
	ErrGroupersResourcesEmptyOrNil   Error = "groupers resources are empty or nil"

	// Admission-related Errors
	ErrCreateValidatingAdmission Error = "error while creating admission validating " +
		"webhook."
	ErrCreateMutatingAdmission Error = "error while creating admission mutating " +
		"webhook."
	ErrGetValidatingAdmission Error = "error while fetching admission validating " +
		"webhook."
	ErrGetMutatingAdmission Error = "error while fetching admission mutating " +
		"webhook."
	ErrUpdateValidatingAdmission Error = "error while updating admission validating " +
		"webhook."
	ErrUpdateMutatingAdmission Error = "error while updating admission mutating " +
		"webhook."
	ErrDeleteValidatingAdmission Error = "error while deleting admission validating " +
		"webhook."
	ErrDeleteMutatingAdmission Error = "error while deleting admission mutating " +
		"webhook."
	ErrFilterAdmission         Error = "error while filtering admission webhook."
	ErrCheckAdmissionExistence Error = "error while checking admission webhook " +
		"existence."
	ErrAdmissionValidatingExists Error = "admission validating webhook already exists."
	ErrAdmissionMutatingExists   Error = "admission mutating webhook already exists."
	ErrAdmissionNotFound         Error = "admission webhook not found."

	// NATS-related Errors
	ErrNatsAuth                               Error = "user and password must be set for auth"
	ErrNatsConnectionAttemptAlreadyInProgress Error = "connection attempt already in progress"
	ErrNatsConnectionLost                     Error = "nats connection lost,triggering reconnection"
	ErrNatsConnectionFailedWithRetry          Error = "nats connection failed.retrying in %s...: %v"
	ErrNatsConnectionFailed                   Error = "nats connection failed: %v"
	ErrNatsClientNotAvailable                 Error = "nats client not available"
	ErrNatsInvalidURL                         Error = "invalid nats server URL"
	ErrNatsTopicNotFound                      Error = "topic %s was not found"
	ErrNatsTopicPublish                       Error = "failed to publish to topic %s: %v"
	ErrNatsTopicSubscribe                     Error = "failed to subscribe to topic %s: %v"
	ErrNatsSubscriberManager                  Error = "failed to start subscriber manager: %v"
	ErrNatsClientClosed                       Error = "nats client closed"
	ErrNatsAckMsg                             Error = "failed to acknowledge message on topic %s: %v"
	ErrNatsAckDupMsg                          Error = "failed to acknowledge duplicated message"
	ErrNatsFailedToCreateConsumer             Error = "failed to create consumer %s: %v"
	ErrNatsHandleMsg                          Error = "failed to handle message %v :%v"
	ErrNatsInvalidSubject                     Error = "invalid subject format"
	ErrNatsConvertMsgData                     Error = "failed to convert message data to"
	ErrNatsFailedCreateStreams                Error = "failed to create streams: %v"
	ErrNatsFailedCreateStream                 Error = "failed to create stream %s: %v"
	ErrNatsFailedConnectToNatsServer          Error = "failed to connect to NATS server: %v"
	ErrNatsDisconnect                         Error = "error while disconnecting from NATS server"
	ErrNatsClientNotConnected                 Error = "nats client not connected"
	ErrNatsJetstreamNotInitialized            Error = "nats jetstream client not initialized"
	ErrNatsCreateJetstreamContext             Error = "failed to create jetstream context: %v"
	ErrNatsConvertFasid                       Error = "error converting message to fasid"
	ErrNatsPatchGrouper                       Error = "error patching grouper: %s"
	ErrNatsMsgAlreadyAcknowledged             Error = "nats: message was already acknowledged"
	ErrNatsEmptyMsgData                       Error = "empty message data"
	ErrNatsMarshalMsg                         Error = "failed to marshal message: %v"
	ErrNatsHandlerCallbackNotImplemented      Error = "handler callback not implemented"
	ErrNatsNoParsedMessageFoundInMetadata     Error = "no parsed message found in metadata"
	ErrNatsNoScopeFoundInUpdateMessage        Error = "no scope found in update message"
	ErrNatsAckError                           Error = "failed to acknowledge: %v"
	ErrNatsNoFasidOrCacidDataFound            Error = "no fasid or cacid data found in " +
		"message for full scope"
	ErrNatsUnknownScope                     Error = "unknown scope: %s"
	ErrNatsCouldNotDetermineResNameFromData Error = "could not determine %s name from " +
		"message data"
	ErrNatsNoResourceNameFoundInDeleteMessage Error = "no resourceName found in delete message"
	ErrNatsFailedToMarshalMsgData             Error = "failed to marshal message data: %v"
	ErrNatsFailedToUnmarshalMsgData           Error = "failed to unmarshal message data: %v"
	ErrNatsFailedToMarshalWorkloadData        Error = "failed to marshal workload data: %v"
	ErrNatsFailedToPatchGrouper               Error = "failed to patch grouper %s: %s"
	ErrNatsFailedToPatchWorkload              Error = "failed to patch workload %s: %s"
	ErrNatsFailedToPatchBridge                Error = "failed to patch bridge %s: %s"
	ErrNatsFailedToDeleteGrouper              Error = "failed to delete grouper %s: %s"
	ErrNatsFailedToDeleteBridge               Error = "failed to delete bridge %s: %s"
	ErrNatsFailedToDeleteWorkload             Error = "failed to delete workload %s: %s"

	// Common Errors
	ErrInvalidAction       Error = "invalid action."
	ErrUnknown             Error = "unknown error."
	ErrInvalidMessage      Error = "invalid message: nil or empty"
	ErrResourceNameTooLong Error = "resource name too long (max 253 characters)"
	ErrUnsupportedType     Error = "unsupported type: %T"

	// Server-related Errors
	ErrServerFailedToStart Error = "server failed to start"
)
