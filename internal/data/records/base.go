package records

type Record string

const (
	RECORD_RESOURCE_CREATED Record = "Resource Was Created With Success"
	RECORD_FEATURE_ENBALED  Record = "Feature Was Enabled"
	RECORD_FEATURE_DISABLED Record = "Feature Was Disabled"

	RECORD_RESOURCE_UPDATED_SYNC_MODE    Record = "Sync Mode Was Changed To"
	RECORD_RESOURCE_UPDATED_SYNC_PERIOD  Record = "Sync Period Was Changed To"
	RECORD_RESOURCE_UPDATED_GENERAL_INFO Record = "General Informations Was Updated with success"
)
