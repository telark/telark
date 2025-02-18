package banners

type Banner string

const (
	// Common
	POLICY_VIOLATION Banner = "🚨 POLICY VIOLATION 🚨\n\n"

	// Admissions
	BANNER_DENY_CREATE_OPERATION Banner = "Maintenance Mode is currently enabled on namespace {name}. By default, creating new resources is **not** allowed during maintenance."
	BANNER_DENY_UPDATE_OPERATION Banner = "Maintenance Mode is currently enabled on namespace {name}. The current configuration does **not** allow **Updates**. Please Check Your Settings"
	BANNER_DENY_DELETE_OPERATION Banner = "Maintenance Mode is currently enabled on namespace {name}. The current configuration does **not** allow **Deletions**. Please Check Your Settings"
)
