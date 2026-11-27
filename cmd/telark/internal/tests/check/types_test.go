package check

import "github.com/telark/telark/cmd/telark/internal/tests/testutil"

type checkCase struct {
	cluster testutil.Cluster
	args    []string
	lines   []string
	code    int
}
