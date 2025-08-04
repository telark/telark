package banners

type Banner string

const (
	BannerMaintenanceDenyCreateOperation Banner = "🚫 **Maintenance Mode** is currently enabled on namespace `{name}`. By default, creating new resources is **not allowed** during maintenance."
	BannerMaintenanceDenyUpdateOperation Banner = "🚫 **Maintenance Mode** is currently enabled on namespace `{name}`. The current configuration does **not allow updates**. Please check your settings."
	BannerMaintenanceDenyDeleteOperation Banner = "🚫 **Maintenance Mode** is currently enabled on namespace `{name}`. The current configuration does **not allow deletions**. Please check your settings."
	BannerPolicyViolation                Banner = "🚨 POLICY VIOLATION 🚨\n\n"
)
