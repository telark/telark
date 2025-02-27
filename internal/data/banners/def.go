package banners

type Banner string

const (
	// Admission-related Banners
	BANNER_MAINTENANCE_DENY_CREATE_OPERATION Banner = "🚫 **Maintenance Mode** is currently enabled on namespace `{name}`. By default, creating new resources is **not allowed** during maintenance."
	BANNER_MAINTENANCE_DENY_UPDATE_OPERATION Banner = "🚫 **Maintenance Mode** is currently enabled on namespace `{name}`. The current configuration does **not allow updates**. Please check your settings."
	BANNER_MAINTENANCE_DENY_DELETE_OPERATION Banner = "🚫 **Maintenance Mode** is currently enabled on namespace `{name}`. The current configuration does **not allow deletions**. Please check your settings."

	// General Status Banners
	POLICY_VIOLATION Banner = "🚨 POLICY VIOLATION 🚨\n\n"
)
