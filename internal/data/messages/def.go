package messages

type Message string

const (
	// Resources
	SUCCESS_LIST_RESOURCES   Message = "Resources Listed Successfully"
	SUCCESS_GET_RESOURCE     Message = "Resource Fetched successfully"
	SUCCESS_CREATE_RESOURCE  Message = "Resource Created successfully"
	SUCCESS_UPDATE_RESOURCE  Message = "Resource Updated successfully"
	SUCCESS_DELETED_RESOURCE Message = "Resource Deleted successfully"

	// Admissions
	SUCCESS_CREATE_VALIDATING_ADMISSION Message = "Validating Webhook Admission Created successfully"
	SUCCESS_CREATE_MUTATING_ADMISSION   Message = "Mutating Webhook Admission Created successfully"
)
