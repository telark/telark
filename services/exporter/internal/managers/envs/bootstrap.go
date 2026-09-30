package envs

import (
	"strings"

	"github.com/telark/telark/services/exporter/internal/constants"
)

var bootstrapAdminEmail string

// The mailbox auth reserves for the bootstrap admin, compared trimmed and lowercased.
func InitBootstrapAdmin() {
	bootstrapAdminEmail = strings.ToLower(strings.TrimSpace(getEnv(constants.BootstrapAdminEnv)))
}

func GetBootstrapAdmin() string {
	return bootstrapAdminEmail
}
