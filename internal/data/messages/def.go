package messages

type Message string

const (
	// Custom Resource Management Messages
	SUCCESS_LIST_RESOURCES                Message = "resources listed successfully."
	SUCCESS_GET_RESOURCE                  Message = "resource %s of kind %s fetched successfully."
	SUCCESS_CREATE_RESOURCE               Message = "resource %s of kind %s created successfully."
	SUCCESS_UPDATE_RESOURCE               Message = "resource %s of kind %s updated successfully."
	SUCCESS_PATCH_RESOURCE                Message = "resource %s of kind %s patched successfully."
	SUCCESS_DELETE_RESOURCE               Message = "resource %s of kind %s deleted successfully."
	INFO_RESOURCE_CREATE_WITH_OUT_HISTORY Message = "resource %s of kind %s created without add in history."
	INFO_RESOURCE_UPDATE_WITH_OUT_HISTORY Message = "resource %s of kind %s updated without add in history."
	INFO_RESOURCE_PATCH_WITH_OUT_HISTORY  Message = "resource %s of kind %s patched without add in history."
	INFO_RESOURCE_DELETE_WITH_OUT_HISTORY Message = "resource %s of kind %s deleted without add in history."

	// Validating Admission Webhook Messages
	SUCCESS_CREATE_VALIDATING_ADMISSION Message = "admission validating webhook created successfully."
	SUCCESS_GET_VALIDATING_ADMISSION    Message = "admission validating webhook fetched successfully."
	SUCCESS_UPDATE_VALIDATING_ADMISSION Message = "admission validating webhook updated successfully."
	SUCCESS_DELETE_VALIDATING_ADMISSION Message = "admission validating webhook deleted successfully."

	// Mutating Admission Webhook Messages
	SUCCESS_CREATE_MUTATING_ADMISSION Message = "admission mutating webhook created successfully."
	SUCCESS_GET_MUTATING_ADMISSION    Message = "admission mutating webhook fetched successfully."
	SUCCESS_UPDATE_MUTATING_ADMISSION Message = "admission mutating webhook updated successfully."
	SUCCESS_DELETE_MUTATING_ADMISSION Message = "admission mutating webhook deleted successfully."

	// General Status Messages
	SUCCESS_OPERATION Message = "operation completed successfully."
	SUCCESS_CREATION  Message = "creation completed successfully."
	SUCCESS_DELETION  Message = "deletion completed successfully."
	SUCCESS_UPDATE    Message = "update completed successfully."

	// NATS
	SUCCESS_NATS_CONNECTION_STATUS_CONNECTED    Message = "nats connection status: connected"
	SUCCESS_NATS_CONNECTION_STATUS_DISCONNECTED Message = "nats connection status: disconnected"
	SUCCESS_NATS_TOPIC_SUBSCRIBE                Message = "successfully subscribed to topic %s"
	SUCCESS_NATS_TOPIC_PUBLISH                  Message = "successfully published to topic %s"
	SUCCESS_NATS_TOPIC_MESSAGE_RECEIVE          Message = "received message on topic %s"
	SUCCESS_NATS_TOPIC_MESSAGE_SEND             Message = "sending message on topic %s"
	SUCCESS_NATS_ALL_SUBSCRIBERS_STARTED        Message = "all subscribers started successfully"
	SUCCESS_NATS_PATCH_GROUPER                  Message = "successfully patched grouper %s"
	INFO_SKIPPING_ACK_MESSAGE                   Message = "skipping acknowledgment message: %s"
	INFO_PROCESSING_MESSAGE                     Message = "processing message: %s"
	INFO_SKIPPING_DUPLICATE                     Message = "skipping duplicate message (processed %v ago)"
	ACK_SENT_FOR_MESSAGE                        Message = "ack sent for message: %s"
	SUCCESS_NATS_DELETE_GROUPER                 Message = "successfully deleted grouper %s"
	SUCCESS_NATS_DELETE_BRIDGE                  Message = "successfully deleted bridge %s"
	SUCCESS_NATS_DELETE_WORKLOAD                Message = "successfully deleted workload %s"
	SUCCESS_NATS_PATCH_WORKLOAD                 Message = "successfully patched workload %s"
	SUCCESS_NATS_PATCH_BRIDGE                   Message = "successfully patched bridge %s"

	// Common Server Messages
	SUCCESS_RECEIVED_SHUTDOWN_SIG Message = "received shutdown signal, cleaning up..."
	SUCCESS_CONTEXT_CANCELED      Message = "context canceled, cleaning up..."
	SUCCESS_STARTING_SERVER       Message = "starting server on port"

	// Health Monitoring
	SUCCESS_SERVICE_HEALTH_CHECK_PASSED Message = "service health check passed"
	SUCCESS_SERVICE_RESTARTING          Message = "restarting service due to health check failure..."

	// Patch-related Messages
	INFO_BRIDGE_OR_WORKLOADS_ARE_EMPTY                      Message = "bridge or workloads are empty"
	INFO_BATCH_WORKLOAD_PATCHING_NOT_SUPPORTED_YET          Message = "batch workload patching operation not supported yet"
	INFO_SKIPPING_PATCHING_EMPTY_APPS_OR_BRIDGES_COLLECTION Message = "skipping patching: empty apps or bridges collection."

	// History Records
	RECORD_RESOURCE_CREATED Message = "resource created successfully"
	RECORD_RESOURCE_UPDATED Message = "resource updated successfully"
)
