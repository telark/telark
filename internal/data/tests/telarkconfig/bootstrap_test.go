package telarkconfig

import (
	"encoding/json"
	"testing"

	"github.com/telark/telark/internal/data/resources/telarkconfig"
)

const (
	requester       = "system:serviceaccount:telark:ops-admin"
	requesterJSON   = `{"id":"","kind":"","requestedBy":"system:serviceaccount:telark:ops-admin","state":"","openedAt":""}`
	bareRequestJSON = `{"id":"","kind":"","state":"","openedAt":""}`
	alertsJSON      = `{"authAlerts":{"notifyAdmins":true}}`
)

// Nothing is pinned without pinnedAt, the newest request is the first one kept, and only a pending or approved
// request is open.
func TestBootstrapStatus(t *testing.T) {
	var none *telarkconfig.BootstrapStatus
	if none.Pinned() || none.Newest() != nil {
		t.Fatal("a missing status is pinned or has a request")
	}
	unpinned := &telarkconfig.BootstrapStatus{UserID: "u-1", Email: "test@example.com"}
	if unpinned.Pinned() || unpinned.Newest() != nil {
		t.Fatal("a status without pinnedAt is pinned or has a request")
	}
	status := &telarkconfig.BootstrapStatus{PinnedAt: "2026-10-16T00:00:00Z", Requests: []telarkconfig.BootstrapRequest{
		{ID: "newest", State: telarkconfig.BootstrapStatePending}, {ID: "older", State: telarkconfig.BootstrapStateCompleted},
	}}
	if !status.Pinned() || status.Newest().ID != "newest" {
		t.Fatalf("pinned %v, newest %+v", status.Pinned(), status.Newest())
	}
	for state, open := range map[string]bool{
		telarkconfig.BootstrapStatePending:   true,
		telarkconfig.BootstrapStateApproved:  true,
		telarkconfig.BootstrapStateCanceled:  false,
		telarkconfig.BootstrapStateCompleted: false,
	} {
		if got := (&telarkconfig.BootstrapRequest{State: state}).Open(); got != open {
			t.Errorf("%s open = %v, want %v", state, got, open)
		}
	}
}

// The stored names the CRD declares: the alerts block, and who opened a request, left out when unknown.
func TestBootstrapWireNames(t *testing.T) {
	cases := []struct {
		name   string
		record any
		want   string
	}{
		{"a request with its requester", telarkconfig.BootstrapRequest{RequestedBy: requester}, requesterJSON},
		{"a request stored before requesters", telarkconfig.BootstrapRequest{}, bareRequestJSON},
		{"the alerts block", map[string]any{telarkconfig.FieldAuthAlerts: telarkconfig.AuthAlertsConfig{NotifyAdmins: true}}, alertsJSON},
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
