package tests

import (
	"testing"

	"github.com/telark/auth/cmd"
	"github.com/telark/auth/internal/tests/testutil"
)

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
