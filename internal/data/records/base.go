package records

type Record string

const (
	RecordUpdateResourceSyncMode    Record = "Sync mode was changed to"
	RecordUpdateResourceSyncPeriod  Record = "Sync period was changed to"
	RecordUpdateResourceGeneralInfo Record = "General informations was updated."
	RecordEnableMaintenanceFeat     Record = "Maintenance was Enabled"
	RecordDisableMaintenanceFeat    Record = "Maintenance was Disabled"
	RecordUpdateMaintenanceFeat     Record = "Maintenance was Updated"
	RecordRemoveMaintenanceFeat     Record = "Maintenance was Removed"
)
