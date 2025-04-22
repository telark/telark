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
)
