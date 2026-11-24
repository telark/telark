package root

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/telark/telark/cmd/telark/internal/auth"
	"github.com/telark/telark/cmd/telark/internal/check"
	"github.com/telark/telark/cmd/telark/internal/constants"
	"github.com/telark/telark/cmd/telark/internal/install"
	"github.com/telark/telark/cmd/telark/internal/kube"
	"github.com/telark/telark/cmd/telark/internal/version"
)

func Run(ctx context.Context, stdout, stderr io.Writer, args []string) int {
	cmd := newCommand(stdout, stderr)
	cmd.SetArgs(args)
	if err := cmd.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintf(stderr, constants.ErrorFormat, err)
		return constants.ExitCodeFailure
	}
	return constants.ExitCodeSuccess
}

func newCommand(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           constants.CommandName,
		Short:         constants.RootShort,
		Long:          constants.RootLong,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	kubeFlags := kube.AddFlags(cmd)
	cmd.AddCommand(version.NewCommand(kubeFlags), check.NewCommand(kubeFlags),
		install.NewInstallCommand(kubeFlags), install.NewUpgradeCommand(kubeFlags), auth.NewCommand(kubeFlags))
	return cmd
}
