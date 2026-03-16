package messages

type Message string

const (
	// Resource-related Messages
	SuccessListRes                  Message = "resources listed successfully"
	SuccessGetRes                   Message = "resource %s of kind %s fetched successfully"
	SuccessCreateRes                Message = "resource %s of kind %s created successfully"
	SuccessUpdateRes                Message = "resource %s of kind %s updated successfully"
	SuccessPatchRes                 Message = "resource %s of kind %s patched successfully"
	SuccessDeleteRes                Message = "resource %s of kind %s deleted successfully"
	InfResCreateWithoutHistory      Message = "resource %s of kind %s created without add in history"
	InfResUpdateWithoutHistory      Message = "resource %s of kind %s updated without add in history"
	InfResPatchWithoutHistory       Message = "resource %s of kind %s patched without add in history"
	InfResDeleteWithoutHistory      Message = "resource %s of kind %s deleted without add in history"
	InfBridgeOrWorkloadsAreEmpty    Message = "bridge or workloads are empty"
	InfBatchWorkloadPatchNotSuppYet Message = "batch workload patching operation not supported yet"
	InfSkipPatchEmptyAppsOrBridges  Message = "skipping patching: empty apps or bridges collection"
	SuccessRecordResCreated         Message = "resource created successfully"
	SuccessRecordResUpdated         Message = "resource updated successfully"

	// Admission-related Messages
	SuccessCreateAdmission Message = "admission %s:%s-webhook created successfully"
	SuccessGetAdmission    Message = "admission %s:%s-webhook fetched successfully"
	SuccessUpdateAdmission Message = "admission %s:%s-webhook updated successfully"
	SuccessDeleteAdmission Message = "admission %s:%s-webhook deleted successfully"

	// Status Messages
	SuccessOperation Message = "operation completed successfully"
	SuccessCreation  Message = "creation completed successfully"
	SuccessDeletion  Message = "deletion completed successfully"
	SuccessUpdate    Message = "update completed successfully"

	// NATS-related Messages
	SuccessNatsConnectionStatusConnected    Message = "nats status: connected"
	SuccessNatsConnectionStatusDisconnected Message = "nats status: disconnected"
	SuccessNatsTopicSubscribe               Message = "successfully subscribed to topic %s"
	SuccessNatsTopicPublish                 Message = "successfully published to topic %s"
	SuccessNatsTopicMessageReceive          Message = "received message on topic %s"
	SuccessNatsTopicMessageSend             Message = "sending message on topic %s"
	SuccessNatsAllSubscribersStarted        Message = "all subscribers started successfully"
	SuccessNatsPatchGrouper                 Message = "successfully patched grouper %s"
	InfoSkippingAckMessage                  Message = "skipping acknowledgment message: %s"
	InfoProcessingMessage                   Message = "processing message: %s"
	InfoSkippingDuplicate                   Message = "skipping duplicate message (pro %v ago)"
	InfoAckSentForMessage                   Message = "ack sent for message: %s"
	SuccessNatsDeleteGrouper                Message = "successfully deleted grouper %s"
	SuccessNatsDeleteBridge                 Message = "successfully deleted bridge %s"
	SuccessNatsDeleteWorkload               Message = "successfully deleted workload %s"
	SuccessNatsPatchWorkload                Message = "successfully patched workload %s"
	SuccessNatsPatchBridge                  Message = "successfully patched bridge %s"
	SuccessNatsPatchApplication             Message = "successfully patched application %s"
	SuccessNatsDeleteApplication            Message = "successfully deleted application %s"

	// Server-related Messages
	SuccessReceivedShutdownSig      Message = "received shutdown signal, cleaning up..."
	SuccessContextCanceled          Message = "context canceled, cleaning up..."
	SuccessStartingServer           Message = "starting server on port"
	SuccessServiceHealthCheckPassed Message = "service health check passed"
	SuccessServiceRestarting        Message = "restarting service due to health check failure..."
)
