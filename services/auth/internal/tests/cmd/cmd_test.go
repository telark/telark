package cmd

import (
	"testing"

	"github.com/telark/auth/internal/cmd"
	"github.com/telark/auth/internal/tests/testutil"
)

// Dispatch only claims an invocation when the first arg names a known
// subcommand; otherwise it hands control back to the normal service boot.
func TestDispatch(t *testing.T) {
	cases := []struct {
		name        string
		args        []string
		wantHandled bool
	}{
		{"no args", nil, false},
		{"binary only", []string{"auth"}, false},
		{"unknown subcommand", []string{"auth", "nope"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			handled, _ := cmd.Dispatch(c.args)
			testutil.Equal(t, "handled", handled, c.wantHandled)
		})
	}
}
