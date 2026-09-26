package envs

import (
	"strings"

	"github.com/telark/exporter/internal/constants"
)

var bootstrapAdminEmails []string

// The same list auth promotes on first login; an entry is one mailbox, so it
// is compared trimmed and lowercased.
func InitBootstrapAdmins() []string {
	bootstrapAdminEmails = bootstrapAdminEmails[:constants.DefaultInitValue]
	for _, entry := range strings.Split(getEnv(constants.BootstrapAdminsEnv), constants.CommaSeparator) {
		if email := strings.ToLower(strings.TrimSpace(entry)); email != constants.EmptyString {
			bootstrapAdminEmails = append(bootstrapAdminEmails, email)
		}
	}
	return bootstrapAdminEmails
}

func GetBootstrapAdmins() []string {
	return bootstrapAdminEmails
}
