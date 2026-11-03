package planupdate

import (
	"context"
	"net/http"
	"testing"

	"github.com/telark/telark/internal/data/plans"
	userresource "github.com/telark/telark/internal/data/resources/user"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/update"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/validation"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	knownParticipant   = "u-release-lead"
	removedParticipant = "u-removed"
	unknownParticipant = "u-unknown"
)

// An edit that adds a participant who is no user is refused before any patch or cluster call; one already on the
// plan is kept even if the user was deleted since.
func TestRunRefusesAnUnknownAddedParticipant(t *testing.T) {
	plan, live := activeNamespacesPlan(t, plans.ModeAudit, tplBlockCreate)
	plan.ParticipantRefs = []string{removedParticipant}
	req := requestFor(plan)
	req.ParticipantRefs = []string{removedParticipant, knownParticipant, unknownParticipant}
	stub := &exporterStub{plan: *plan, patchStatus: http.StatusOK}
	stub.install(t)
	dyn := clusterWith(t, live...)
	deps := runDeps(dyn, &reportsRecorder{})
	deps.Users = func() ([]*userresource.User, error) { return []*userresource.User{{ID: knownParticipant}}, nil }

	_, err := update.Run(context.Background(), deps, userID, plan.ID, req)
	testutil.Equal(t, "validation error", validation.IsValidation(err), true)
	testutil.Equal(t, labelPatches, len(stub.patches), constants.DefaultInitValue)
	testutil.Equal(t, "cluster calls", len(dyn.Actions()), constants.DefaultInitValue)
}
