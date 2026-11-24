package check

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/telark/telark/cmd/telark/internal/constants"
	"github.com/telark/telark/cmd/telark/internal/kube"
	"github.com/telark/telark/cmd/telark/internal/output"
	"github.com/telark/telark/cmd/telark/internal/version"
	"k8s.io/client-go/kubernetes"
)

var errModeNeedsPre = errors.New(constants.ErrModeNeedsPre)

func NewCommand(kubeFlags *kube.Flags) *cobra.Command {
	opts := &options{kube: kubeFlags}
	cmd := &cobra.Command{
		Use:   constants.CheckUse,
		Short: constants.CheckShort,
		Long:  constants.CheckLong,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := opts.validate(cmd.Flags().Changed(constants.FlagMode)); err != nil {
				return err
			}
			return opts.run(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	flags := cmd.Flags()
	flags.BoolVar(&opts.pre, constants.FlagPre, false, constants.FlagPreUsage)
	flags.DurationVar(&opts.wait, constants.FlagWait, constants.DefaultInitValue, constants.FlagWaitUsage)
	flags.BoolVar(&opts.skipUpdate, constants.FlagSkipUpdateCheck, false, constants.FlagSkipUpdateCheckUsage)
	flags.StringVar(&opts.mode, constants.FlagMode, constants.ModeStandard, constants.FlagModeUsage)
	_ = cmd.RegisterFlagCompletionFunc(constants.FlagMode, cobra.FixedCompletions(modes, cobra.ShellCompDirectiveNoFileComp))
	output.AddFormatFlag(cmd, &opts.format)
	return cmd
}

func (o *options) validate(modeSet bool) error {
	if modeSet && !o.pre {
		return errModeNeedsPre
	}
	if _, found := modeSizes[o.mode]; !found {
		return fmt.Errorf(constants.ErrUnknownMode, o.mode)
	}
	return nil
}

func (o *options) run(ctx context.Context, stdout, stderr io.Writer) error {
	client, err := o.kube.Clientset()
	if err != nil {
		return err
	}
	opts := Options{Pre: o.pre, Wait: o.wait, Mode: o.mode, UpdateCheck: o.updateCheck()}
	report := Run(ctx, client, o.kube.Namespace, stderr, opts)
	if err := report.Write(stdout, o.format); err != nil {
		return err
	}
	return report.Err()
}

// The update check runs once, outside the loop, so a wait never repeats the call to GitHub.
func Run(ctx context.Context, client kubernetes.Interface, namespace string, stderr io.Writer, opts Options) Report {
	if opts.Wait > constants.DefaultInitValue {
		_, _ = fmt.Fprintf(stderr, constants.MsgWaiting, opts.Wait)
	}
	r := &runner{core: client, namespace: namespace, opts: opts}
	deadline := time.Now().Add(opts.Wait)
	results := r.run(ctx)
	for remaining := time.Until(deadline); slices.ContainsFunc(results, failed) && remaining > constants.DefaultInitValue; {
		time.Sleep(min(constants.CheckPollInterval, remaining))
		results, remaining = r.run(ctx), time.Until(deadline)
	}
	if opts.UpdateCheck {
		results = append(results, latestRelease(ctx, version.Client().Version))
	}
	return NewReport(results)
}

func NewReport(results []Result) Report {
	return Report{Success: !slices.ContainsFunc(results, failed), Checks: results}
}

func (o *options) updateCheck() bool {
	skip, _ := strconv.ParseBool(os.Getenv(constants.EnvSkipUpdateCheck))
	return !o.skipUpdate && !skip
}

func (r *runner) run(ctx context.Context) []Result {
	if r.opts.Pre {
		return r.preinstall(ctx)
	}
	return r.installed(ctx)
}
