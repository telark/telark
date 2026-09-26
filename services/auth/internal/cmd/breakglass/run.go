package breakglass

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/telark/auth/internal/clients"
	"github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
	authhelper "github.com/telark/auth/internal/helpers/auth"
)

// Operator-run, so the email is trusted; a BOOTSTRAP_ADMINS address also gets the
// chart marker, which is how a bootstrap user created before it existed is marked.
func Run(args []string) int {
	fs := flag.NewFlagSet(constants.BreakGlassFlagSet, flag.ExitOnError)
	email := fs.String(constants.BreakGlassFlagEmail, constants.EmptyString, constants.BreakGlassFlagEmailUsage)
	if err := fs.Parse(args); err != nil || *email == constants.EmptyString {
		fmt.Fprintln(os.Stderr, constants.BreakGlassUsage)
		return constants.ExitCodeError
	}

	normalized := strings.ToLower(strings.TrimSpace(*email))
	userClient := clients.GetUserClient()

	user, err := userClient.GetUserByEmail(normalized)
	if err != nil || user == nil {
		fmt.Fprintf(os.Stderr, constants.BreakGlassUserNotFound, normalized)
		return constants.ExitCodeError
	}

	patch := map[string]any{}
	if !authhelper.HasAdminRole(user.AssignedRolesIDs) {
		adminID := constants.BuiltInRoleAdmin
		patch[constants.SpecFieldAssignedRolesIDs] = append(slices.Clone(user.AssignedRolesIDs), &adminID)
	}
	if _, cfgErr := config.LoadBootstrapConfig(); cfgErr == nil && config.IsBootstrapAdmin(normalized) && !user.Bootstrap {
		patch[constants.UserFieldBootstrap] = true
	}
	if len(patch) == constants.DefaultInitValue {
		fmt.Printf(constants.BreakGlassAlreadyAdmin, normalized)
		return constants.DefaultInitValue
	}

	resp := userClient.PatchUserByID(user.ID, patch)
	if resp.Status != http.StatusOK {
		fmt.Fprintf(os.Stderr, constants.BreakGlassPatchFailed, resp.Status, resp.Message)
		return constants.ExitCodeError
	}

	fmt.Printf(constants.BreakGlassPromoted, normalized)
	return constants.DefaultInitValue
}
