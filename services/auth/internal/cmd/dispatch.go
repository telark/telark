package cmd

import (
	"github.com/telark/auth/internal/cmd/backfill"
	"github.com/telark/auth/internal/cmd/breakglass"
	"github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
)

const (
	SubcommandBreakGlass         = "break-glass"
	SubcommandBackfillFinalizers = "backfill-finalizers"
)

type runner func(args []string) int

var registry = map[string]runner{
	SubcommandBreakGlass:         breakglass.Run,
	SubcommandBackfillFinalizers: backfillRunner,
}

func Dispatch(args []string) (handled bool, exitCode int) {
	if len(args) <= constants.DefaultIncrementValue {
		return false, constants.DefaultInitValue
	}
	run, ok := registry[args[constants.DefaultIncrementValue]]
	if !ok {
		return false, constants.DefaultInitValue
	}
	rest := args[constants.DefaultIncrementValue+constants.DefaultIncrementValue:]
	return true, run(rest)
}

func backfillRunner(_ []string) int {
	cfg := config.LoadBackfillConfig()
	lg := constants.GetLogger(constants.LoggerPrefixCleanup)
	if err := backfill.Run(cfg, lg); err != nil {
		return constants.ExitCodeError
	}
	return constants.DefaultInitValue
}
