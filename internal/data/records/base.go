package records

type Record string

const (
	// Resource-related Records
	RECORD_UPDATE_RESOURCE_SYNC_MODE    Record = "Sync mode was changed to"
	RECORD_UPDATE_RESOURCE_SYNC_PERIOD  Record = "Sync period was changed to"
	RECORD_UPDATE_RESOURCE_GENERAL_INFO Record = "General informations was updated."

	// Feature-related Records
	RECORD_ENABLE_MAINTENANCE_FEAT  Record = "Maintenance was Enabled"
	RECORD_DISABLE_MAINTENANCE_FEAT Record = "Maintenance was Disabled"
	RECORD_UPDATE_MAINTENANCE_FEAT  Record = "Maintenance was Updated"
	RECORD_REMOVE_MAINTENANCE_FEAT  Record = "Maintenance was Removed"
)
