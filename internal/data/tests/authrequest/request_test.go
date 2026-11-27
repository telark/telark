package authrequest

import (
	"encoding/json"
	"testing"

	"github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/data/resources/authrequest"
)

const (
	requestName = "req-1"
	email       = "test@example.com"
	requestedBy = "system:serviceaccount:telark:ops-admin"
	createdAt   = "2026-11-02T10:00:00Z"
	flatJSON    = `{"id":"req-1","email":"test@example.com","requestedBy":"system:serviceaccount:telark:ops-admin",` +
		`"recipientKey":"","phase":"LinkReady","createdAt":"2026-11-02T10:00:00Z"}`
	phaseOnlyJSON = `{"phase":"Completed"}`
)

// Only a run that stopped halfway leaves a request in flight: one not started and one finished are not.
func TestInFlight(t *testing.T) {
	phases := map[string]bool{
		constants.EmptyString:               false,
		authrequest.PhasePending:            false,
		authrequest.PhaseWaitingForApproval: false,
		authrequest.PhaseRecovering:         true,
		authrequest.PhaseLinkReady:          true,
		authrequest.PhaseCompleted:          false,
		authrequest.PhaseDenied:             false,
		authrequest.PhaseCanceled:           false,
		authrequest.PhaseExpired:            false,
		authrequest.PhaseFailed:             false,
	}
	for phase, want := range phases {
		request := authrequest.AuthRequest{Status: authrequest.Status{Phase: phase}}
		if got := request.InFlight(); got != want {
			t.Errorf("%q in flight = %v, want %v", phase, got, want)
		}
	}
}

// The REST view is flat, and a status carries only the fields a step sets, so the exporter's merge keeps the rest.
func TestWireShape(t *testing.T) {
	cases := []struct {
		name   string
		record any
		want   string
	}{
		{"a request", authrequest.AuthRequest{
			ID:        requestName,
			Spec:      authrequest.Spec{Email: email, RequestedBy: requestedBy},
			Status:    authrequest.Status{Phase: authrequest.PhaseLinkReady},
			CreatedAt: createdAt,
		}, flatJSON},
		{"a status with one field set", authrequest.Status{Phase: authrequest.PhaseCompleted}, phaseOnlyJSON},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := json.Marshal(c.record)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != c.want {
				t.Errorf("JSON = %s, want %s", got, c.want)
			}
		})
	}
}
