package messages

type Message string

const (
	// Custom Resource Management Messages
	SUCCESS_LIST_RESOURCES  Message = "Custom Resources listed successfully."
	SUCCESS_GET_RESOURCE    Message = "Custom Resource fetched successfully."
	SUCCESS_CREATE_RESOURCE Message = "Custom Resource created successfully."
	SUCCESS_UPDATE_RESOURCE Message = "Custom Resource updated successfully."
	SUCCESS_DELETE_RESOURCE Message = "Custom Resource deleted successfully."

	// Validating Admission Webhook Messages
	SUCCESS_CREATE_VALIDATING_ADMISSION Message = "Admission Validating Webhook created successfully."
	SUCCESS_GET_VALIDATING_ADMISSION    Message = "Admission Validating Webhook fetched successfully."
	SUCCESS_UPDATE_VALIDATING_ADMISSION Message = "Admission Validating Webhook updated successfully."
	SUCCESS_DELETE_VALIDATING_ADMISSION Message = "Admission Validating Webhook deleted successfully."

	// Mutating Admission Webhook Messages
	SUCCESS_CREATE_MUTATING_ADMISSION Message = "Admission Mutating Webhook created successfully."
	SUCCESS_GET_MUTATING_ADMISSION    Message = "Admission Mutating Webhook fetched successfully."
	SUCCESS_UPDATE_MUTATING_ADMISSION Message = "Admission Mutating Webhook updated successfully."
	SUCCESS_DELETE_MUTATING_ADMISSION Message = "Admission Mutating Webhook deleted successfully."

	// General Status Messages
	SUCCESS_OPERATION Message = "Operation completed successfully."
	SUCCESS_CREATION  Message = "Creation completed successfully."
	SUCCESS_DELETION  Message = "Deletion completed successfully."
	SUCCESS_UPDATE    Message = "Update completed successfully."

	// NATS
	SUCCESS_NATS_CONNECTION_STATUS_CONNECTED    Message = "NATS connection status: CONNECTED"
	SUCCESS_NATS_CONNECTION_STATUS_DISCONNECTED Message = "NATS connection status: DISCONNECTED"
	SUCCESS_NATS_TOPIC_SUBSCRIBE                Message = "Successfully subscribed to topic %s"
	SUCCESS_NATS_TOPIC_PUBLISH                  Message = "Successfully published to topic %s"
	SUCCESS_NATS_TOPIC_MESSAGE_RECEIVE          Message = "Received message on topic %s"
	SUCCESS_NATS_TOPIC_MESSAGE_SEND             Message = "Sending message on topic %s"
	SUCCESS_NATS_ALL_SUBSCRIBERS_STARTED        Message = "All subscribers started successfully"
	SUCCESS_NATS_PATCH_GROUPER                  Message = "Successfully patched grouper %s"
	INFO_SKIPPING_ACK_MESSAGE                   Message = "Skipping acknowledgment message: %s"
	INFO_PROCESSING_MESSAGE                     Message = "Processing message: %s"
	INFO_SKIPPING_DUPLICATE                     Message = "Skipping duplicate message (processed %v ago)"
	ACK_SENT_FOR_MESSAGE                        Message = "Ack sent for message: %s"
	SUCCESS_NATS_DELETE_GROUPER                 Message = "Successfully deleted grouper %s"
	SUCCESS_NATS_DELETE_BRIDGE                  Message = "Successfully deleted bridge %s"
	SUCCESS_NATS_DELETE_WORKLOAD                Message = "Successfully deleted workload %s"
	SUCCESS_NATS_PATCH_WORKLOAD                 Message = "Successfully patched workload %s"
	SUCCESS_NATS_PATCH_BRIDGE                   Message = "Successfully patched bridge %s"

	// Common Server Messages
	SUCCESS_RECEIVED_SHUTDOWN_SIG Message = "Received shutdown signal, cleaning up..."
	SUCCESS_CONTEXT_CANCELED      Message = "Context canceled, cleaning up..."
	SUCCESS_STARTING_SERVER       Message = "Starting server on port"

	// Health Monitoring
	SUCCESS_SERVICE_HEALTH_CHECK_PASSED Message = "Service health check passed"
	SUCCESS_SERVICE_RESTARTING          Message = "Restarting service due to health check failure..."

	// Patch-related Messages
	INFO_BRIDGE_OR_WORKLOADS_ARE_EMPTY                      Message = "Bridge or Workloads are empty"
	INFO_BATCH_WORKLOAD_PATCHING_NOT_SUPPORTED_YET          Message = "Batch Workload Patching Operation Not Supported Yet"
	INFO_SKIPPING_PATCHING_EMPTY_APPS_OR_BRIDGES_COLLECTION Message = "Skipping patching: empty apps or bridges collection."
)
