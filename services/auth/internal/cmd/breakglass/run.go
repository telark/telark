package breakglass

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/telark/auth/internal/clients"
	"github.com/telark/auth/internal/constants"
)

func Run(args []string) int {
	fs := flag.NewFlagSet("break-glass", flag.ExitOnError)
	email := fs.String("email", constants.EmptyString, "Email of the user to promote to Admin")
	if err := fs.Parse(args); err != nil || *email == constants.EmptyString {
		fmt.Fprintln(os.Stderr, "usage: auth break-glass --email <email>")
		return constants.ExitCodeError
	}

	normalized := strings.ToLower(strings.TrimSpace(*email))
	userClient := clients.GetUserClient()

	user, err := userClient.GetUserByEmail(normalized)
	if err != nil || user == nil {
		fmt.Fprintf(os.Stderr, "user not found: %s\n", normalized)
		return constants.ExitCodeError
	}

	for _, rid := range user.AssignedRolesIDs {
		if rid != nil && *rid == constants.BuiltInRoleAdmin {
			fmt.Printf("user %s already has Admin role\n", normalized)
			return constants.DefaultInitValue
		}
	}

	adminID := constants.BuiltInRoleAdmin
	roles := make([]*string, constants.InitialCapacity, len(user.AssignedRolesIDs)+constants.DefaultIncrementValue)
	roles = append(roles, user.AssignedRolesIDs...)
	roles = append(roles, &adminID)

	resp := userClient.PatchUserByID(user.ID, map[string]any{constants.SpecFieldAssignedRolesIDs: roles})
	if resp.Status != http.StatusOK {
		fmt.Fprintf(os.Stderr, "patch failed: status %d: %s\n", resp.Status, resp.Message)
		return constants.ExitCodeError
	}

	fmt.Printf("promoted %s to Admin\n", normalized)
	return constants.DefaultInitValue
}
