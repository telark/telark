package breakglass

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/telark/auth/internal/clients"
	"github.com/telark/auth/internal/constants"
	authhelper "github.com/telark/auth/internal/helpers/auth"
)

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

	if authhelper.HasAdminRole(user.AssignedRolesIDs) {
		fmt.Printf(constants.BreakGlassAlreadyAdmin, normalized)
		return constants.DefaultInitValue
	}

	adminID := constants.BuiltInRoleAdmin
	roles := make([]*string, constants.InitialCapacity, len(user.AssignedRolesIDs)+constants.DefaultIncrementValue)
	roles = append(roles, user.AssignedRolesIDs...)
	roles = append(roles, &adminID)

	resp := userClient.PatchUserByID(user.ID, map[string]any{constants.SpecFieldAssignedRolesIDs: roles})
	if resp.Status != http.StatusOK {
		fmt.Fprintf(os.Stderr, constants.BreakGlassPatchFailed, resp.Status, resp.Message)
		return constants.ExitCodeError
	}

	fmt.Printf(constants.BreakGlassPromoted, normalized)
	return constants.DefaultInitValue
}
