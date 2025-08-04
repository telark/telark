package messages

type Message string

const (
	SuccessListResources             Message = "resources listed successfully."
	SuccessGetResource               Message = "resource %s of kind %s fetched successfully."
	SuccessCreateResource            Message = "resource %s of kind %s created successfully."
	SuccessUpdateResource            Message = "resource %s of kind %s updated successfully."
	SuccessPatchResource             Message = "resource %s of kind %s patched successfully."
	SuccessDeleteResource            Message = "resource %s of kind %s deleted successfully."
	InfoResourceCreateWithOutHistory Message = "resource %s of kind %s created without add in history."
	InfoResourceUpdateWithOutHistory Message = "resource %s of kind %s updated without add in history."
	InfoResourcePatchWithOutHistory  Message = "resource %s of kind %s patched without add in history."
	InfoResourceDeleteWithOutHistory Message = "resource %s of kind %s deleted without add in history."

	// Validating Admission Webhook Messages
	SuccessCreateValidatingAdmission Message = "admission validating webhook created successfully."
	SuccessGetValidatingAdmission    Message = "admission validating webhook fetched successfully."
	SuccessUpdateValidatingAdmission Message = "admission validating webhook updated successfully."
	SuccessDeleteValidatingAdmission Message = "admission validating webhook deleted successfully."

	// Mutating Admission Webhook Messages
	SuccessCreateMutatingAdmission Message = "admission mutating webhook created successfully."
	SuccessGetMutatingAdmission    Message = "admission mutating webhook fetched successfully."
	SuccessUpdateMutatingAdmission Message = "admission mutating webhook updated successfully."
	SuccessDeleteMutatingAdmission Message = "admission mutating webhook deleted successfully."

	// General Status Messages
	SuccessOperation Message = "operation completed successfully."
	SuccessCreation  Message = "creation completed successfully."
	SuccessDeletion  Message = "deletion completed successfully."
	SuccessUpdate    Message = "update completed successfully."

	// NATS
	SuccessNatsConnectionStatusConnected    Message = "nats connection status: connected"
	SuccessNatsConnectionStatusDisconnected Message = "nats connection status: disconnected"
	SuccessNatsTopicSubscribe               Message = "successfully subscribed to topic %s"
	SuccessNatsTopicPublish                 Message = "successfully published to topic %s"
	SuccessNatsTopicMessageReceive          Message = "received message on topic %s"
	SuccessNatsTopicMessageSend             Message = "sending message on topic %s"
	SuccessNatsAllSubscribersStarted        Message = "all subscribers started successfully"
	SuccessNatsPatchGrouper                 Message = "successfully patched grouper %s"
	InfoSkippingAckMessage                  Message = "skipping acknowledgment message: %s"
	InfoProcessingMessage                   Message = "processing message: %s"
	InfoSkippingDuplicate                   Message = "skipping duplicate message (processed %v ago)"
	InfoAckSentForMessage                   Message = "ack sent for message: %s"
	SuccessNatsDeleteGrouper                Message = "successfully deleted grouper %s"
	SuccessNatsDeleteBridge                 Message = "successfully deleted bridge %s"
	SuccessNatsDeleteWorkload               Message = "successfully deleted workload %s"
	SuccessNatsPatchWorkload                Message = "successfully patched workload %s"
	SuccessNatsPatchBridge                  Message = "successfully patched bridge %s"

	// Common Server Messages
	SuccessReceivedShutdownSig Message = "received shutdown signal, cleaning up..."
	SuccessContextCanceled     Message = "context canceled, cleaning up..."
	SuccessStartingServer      Message = "starting server on port"

	// Health Monitoring
	SuccessServiceHealthCheckPassed Message = "service health check passed"
	SuccessServiceRestarting        Message = "restarting service due to health check failure..."

	// Patch-related Messages
	InfoBridgeOrWorkloadsAreEmpty                    Message = "bridge or workloads are empty"
	InfoBatchWorkloadPatchingNotSupportedYet         Message = "batch workload patching operation not supported yet"
	InfoSkippingPatchingEmptyAppsOrBridgesCollection Message = "skipping patching: empty apps or bridges collection."

	// History Records
	RecordResourceCreated Message = "resource created successfully"
	RecordResourceUpdated Message = "resource updated successfully"
)
