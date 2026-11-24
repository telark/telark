package version

import (
	"context"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/telark/telark/cmd/telark/internal/constants"
	"github.com/telark/telark/cmd/telark/internal/kube"
	"github.com/telark/telark/cmd/telark/internal/output"
)

func NewCommand(kubeFlags *kube.Flags) *cobra.Command {
	opts := &options{kube: kubeFlags}
	cmd := &cobra.Command{
		Use:   constants.VersionUse,
		Short: constants.VersionShort,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return opts.run(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	cmd.Flags().BoolVar(&opts.clientOnly, constants.FlagClient, false, constants.FlagClientUsage)
	output.AddFormatFlag(cmd, &opts.format)
	return cmd
}

// The client part prints even when the cluster can't be reached, as kubectl version does.
func (o *options) run(ctx context.Context, stdout, stderr io.Writer) error {
	info := Info{Client: Client()}
	var serverErr error
	if !o.clientOnly {
		info.Server, serverErr = o.lookupServer(ctx)
	}
	if err := o.write(stdout, info); err != nil {
		return err
	}
	if serverErr != nil {
		return serverErr
	}
	if info.Server != nil && Skewed(info.Client.Version, info.Server.Version) {
		warning := fmt.Sprintf(constants.WarnVersionSkew, info.Client.Version, info.Server.Version)
		output.NewPrinter(stderr, output.UseColor(stderr)).Warn(warning)
	}
	return nil
}

func (o *options) lookupServer(ctx context.Context) (*ServerInfo, error) {
	client, err := o.kube.Clientset()
	if err != nil {
		return nil, err
	}
	return Server(ctx, client, o.kube.Namespace)
}

func (o *options) write(w io.Writer, info Info) error {
	if o.format != output.FormatTable {
		return output.Write(w, o.format, info)
	}
	var table strings.Builder
	writeClient(&table, info.Client)
	if info.Server != nil {
		writeServer(&table, info.Server)
	}
	_, err := io.WriteString(w, table.String())
	return err
}

func writeClient(w io.Writer, client ClientInfo) {
	_, _ = fmt.Fprintln(w, constants.TableClient)
	_, _ = fmt.Fprintf(w, constants.TableRowFormat, constants.TableVersion, client.Version)
	_, _ = fmt.Fprintf(w, constants.TableRowFormat, constants.TableGitCommit, client.GitCommit)
	_, _ = fmt.Fprintf(w, constants.TableRowFormat, constants.TableBuildDate, client.BuildDate)
	_, _ = fmt.Fprintf(w, constants.TableRowFormat, constants.TableGoVersion, client.GoVersion)
}

func writeServer(w io.Writer, server *ServerInfo) {
	_, _ = fmt.Fprintln(w, constants.TableServer)
	_, _ = fmt.Fprintf(w, constants.TableRowFormat, constants.TableVersion, server.Version)
	_, _ = fmt.Fprintf(w, constants.TableRowFormat, constants.TableNamespace, server.Namespace)
	_, _ = fmt.Fprintln(w, constants.TableServices)
	for _, name := range slices.Sorted(maps.Keys(server.Services)) {
		_, _ = fmt.Fprintf(w, constants.TableServiceRow, name, server.Services[name])
	}
}
