package install

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/telark/telark/cmd/telark/internal/check"
	"github.com/telark/telark/cmd/telark/internal/constants"
	"github.com/telark/telark/cmd/telark/internal/kube"
	"github.com/telark/telark/cmd/telark/internal/version"
	"helm.sh/helm/v4/pkg/action"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/cli/values"
	helmkube "helm.sh/helm/v4/pkg/kube"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	"k8s.io/client-go/kubernetes"
)

var errBootstrapAdmin = errors.New(constants.ErrBootstrapAdmin)

func NewInstallCommand(kubeFlags *kube.Flags) *cobra.Command {
	opts := &installOptions{kube: kubeFlags}
	cmd := &cobra.Command{
		Use:   constants.InstallUse,
		Short: constants.InstallShort,
		Long:  constants.InstallLong,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return opts.run(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.bootstrapAdmin, constants.FlagBootstrapAdmin, constants.EmptyString, constants.FlagBootstrapAdminUsage)
	flags.StringVar(&opts.mode, constants.FlagMode, constants.EmptyString, constants.FlagInstallModeUsage)
	_ = cmd.RegisterFlagCompletionFunc(constants.FlagMode, cobra.FixedCompletions(check.Modes(), cobra.ShellCompDirectiveNoFileComp))
	addChartFlags(cmd, &opts.values, &opts.version, &opts.chartDirectory, &opts.wait, &opts.dryRun)
	return cmd
}

func NewUpgradeCommand(kubeFlags *kube.Flags) *cobra.Command {
	opts := &upgradeOptions{kube: kubeFlags}
	cmd := &cobra.Command{
		Use:   constants.UpgradeUse,
		Short: constants.UpgradeShort,
		Long:  constants.UpgradeLong,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return opts.run(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	cmd.Flags().BoolVar(&opts.resetValues, constants.FlagResetValues, false, constants.FlagResetValuesUsage)
	addChartFlags(cmd, &opts.values, &opts.version, &opts.chartDirectory, &opts.wait, &opts.dryRun)
	return cmd
}

func addChartFlags(cmd *cobra.Command, vals *values.Options, chartVersion, directory *string, wait, dryRun *bool) {
	flags := cmd.Flags()
	flags.StringArrayVarP(&vals.ValueFiles, constants.FlagValues, constants.FlagValuesShort, nil, constants.FlagValuesUsage)
	flags.StringArrayVar(&vals.Values, constants.FlagSet, nil, constants.FlagSetUsage)
	flags.StringVar(chartVersion, constants.FlagChartVersion, version.Client().Version, constants.FlagChartVersionUsage)
	flags.StringVar(directory, constants.FlagChartDirectory, constants.EmptyString, constants.FlagChartDirectoryUsage)
	flags.BoolVar(wait, constants.FlagHelmWait, false, constants.FlagHelmWaitUsage)
	flags.BoolVar(dryRun, constants.FlagDryRun, false, constants.FlagDryRunUsage)
}

func (o *installOptions) run(ctx context.Context, stdout, stderr io.Writer) error {
	client, err := o.kube.Clientset()
	if err != nil {
		return err
	}
	vals, pre, err := o.settings()
	if err != nil {
		return err
	}
	chart, err := o.prepare(ctx, stdout, check.Run(ctx, client, o.kube.Namespace, stderr, pre))
	if err != nil {
		return err
	}
	released, err := accessor(newInstall(configuration(o.kube), o.kube.Namespace, o.wait, o.dryRun).RunWithContext(ctx, chart, vals))
	if err != nil {
		return fmt.Errorf(constants.ErrInstall, err)
	}
	if o.dryRun {
		return writeDryRun(stdout, released.Manifest(), chart.Metadata.Version, o.kube.Namespace, vals)
	}
	admin := stringAt(vals, constants.KeyApp, constants.KeyAuth, constants.KeyBootstrap, constants.KeyAdmin)
	_, err = fmt.Fprintf(stdout, constants.MsgInstalled, chart.Metadata.Version, o.kube.Namespace, admin)
	return err
}

func (o *installOptions) settings() (map[string]any, check.Options, error) {
	if o.bootstrapAdmin != constants.EmptyString {
		o.values.LiteralValues = append(o.values.LiteralValues, fmt.Sprintf(constants.ValueAdmin, o.bootstrapAdmin))
	}
	if o.mode != constants.EmptyString {
		o.values.LiteralValues = append(o.values.LiteralValues, fmt.Sprintf(constants.ValueMode, o.mode))
	}
	vals, err := merge(&o.values)
	if err != nil {
		return nil, check.Options{}, err
	}
	if stringAt(vals, constants.KeyApp, constants.KeyAuth, constants.KeyBootstrap, constants.KeyAdmin) == constants.EmptyString {
		return nil, check.Options{}, errBootstrapAdmin
	}
	mode := cmp.Or(stringAt(vals, constants.KeyApp, constants.KeyMode), constants.ModeStandard)
	if !slices.Contains(check.Modes(), mode) {
		return nil, check.Options{}, fmt.Errorf(constants.ErrInstallMode, mode)
	}
	kyverno, set := valueAt(vals, constants.KeyKyverno, constants.KeyEnabled).(bool)
	return vals, check.Options{Pre: true, Mode: mode, ExternalKyverno: set && !kyverno,
		StorageClass: stringAt(vals, constants.KeyApp, constants.KeyPersistence, constants.KeyStorageClass)}, nil
}

func (o *installOptions) prepare(ctx context.Context, stdout io.Writer, pre check.Report) (*chartv2.Chart, error) {
	pre.Print(stdout)
	if err := pre.Err(); err != nil {
		return nil, err
	}
	registry, err := newRegistry()
	if err != nil {
		return nil, err
	}
	chart, signature, err := source{version: o.version, directory: o.chartDirectory, registry: registry}.chart(ctx)
	if err != nil {
		return nil, err
	}
	signed := check.NewReport([]check.Result{signature})
	signed.Print(stdout)
	return chart, signed.Err()
}

func (o *upgradeOptions) run(ctx context.Context, stdout, stderr io.Writer) error {
	client, err := o.kube.Clientset()
	if err != nil {
		return err
	}
	cfg := configuration(o.kube)
	vals, err := merge(&o.values)
	if err != nil {
		return err
	}
	chart, storage, err := o.preflight(ctx, stdout, cfg, client, vals)
	if err != nil {
		return err
	}
	if o.dryRun {
		_, err := fmt.Fprintf(stdout, constants.MsgDryRunUpgrade, chart.Metadata.Version)
		return err
	}
	if err := storage.run(ctx, stdout); err != nil {
		return err
	}
	upgrade := newUpgrade(cfg, o.kube.Namespace, o.resetValues, o.wait, false)
	if _, err := upgrade.RunWithContext(ctx, constants.ReleaseName, chart, vals); err != nil {
		return errors.Join(fmt.Errorf(constants.ErrUpgrade, err), storage.recover(ctx))
	}
	if _, err := fmt.Fprintf(stdout, constants.MsgUpgraded, chart.Metadata.Version, o.kube.Namespace); err != nil {
		return err
	}
	after := check.Run(ctx, client, o.kube.Namespace, stderr, check.Options{Wait: constants.UpgradeCheckWait})
	after.Print(stdout)
	return after.Err()
}

func (o *upgradeOptions) preflight(ctx context.Context, stdout io.Writer, cfg *action.Configuration, client kubernetes.Interface,
	vals map[string]any) (*chartv2.Chart, *storageCopy, error) {
	installed, err := installedRelease(cfg, o.kube.Namespace)
	if err != nil {
		return nil, nil, err
	}
	registry, err := newRegistry()
	if err != nil {
		return nil, nil, err
	}
	chart, signature, err := source{version: o.version, directory: o.chartDirectory, registry: registry}.chart(ctx)
	if err != nil {
		return nil, nil, err
	}
	published := func() ([]string, error) { return registry.Tags(constants.ChartRepository) }
	renamed := renamedValues(installed.Config, vals, o.resetValues)
	server, manifest := o.serverChecks(ctx, cfg, client, installed, chart, vals)
	if len(renamed) > constants.DefaultInitValue {
		// Helm's dry run consumed the null that drops the old key, so the upgrade gets it again.
		setValue(vals, nil, constants.KeyApp, constants.KeySecurity, constants.KeyBreakGlass)
	}
	storage, moves := planStorage(ctx, cfg, client, installed, manifest)
	report := check.NewReport(slices.Concat(
		[]check.Result{signature, step(installed.Chart.Metadata.Version, chart.Metadata.Version, published)}, renamed, server, moves,
		[]check.Result{warnOnly(check.RecoveryRequest(ctx, client, o.kube.Namespace)), warnOnly(enforcePlans(ctx, client, o.kube.Namespace))}))
	report.Print(stdout)
	return chart, storage, report.Err()
}

// A client-side release never conflicts with another field manager, so its probe forces: a dry run persists nothing.
func (o *upgradeOptions) serverChecks(ctx context.Context, cfg *action.Configuration, client kubernetes.Interface,
	installed *releasev1.Release, chart *chartv2.Chart, vals map[string]any) (results []check.Result, manifest string) {
	upgrade := newUpgrade(cfg, o.kube.Namespace, o.resetValues, o.wait, true)
	dryRun, err := accessor(upgrade.RunWithContext(ctx, constants.ReleaseName, chart, vals))
	var targets helmkube.ResourceList
	if err == nil {
		targets, err = cfg.KubeClient.Build(strings.NewReader(dryRun.Manifest()), false)
	}
	if err != nil {
		return []check.Result{check.Fail(constants.CheckServerDryRun, fmt.Sprintf(constants.MsgServerRefuses, err))}, constants.EmptyString
	}
	return []check.Result{serverApply(targets, o.kube.Namespace, forced(installed)),
		storedObjects(ctx, client, o.kube.Namespace, chart.Metadata.Version, dryRun.Manifest())}, dryRun.Manifest()
}

func forced(installed *releasev1.Release) bool {
	return installed.ApplyMethod != string(releasev1.ApplyMethodServerSideApply)
}

// 0.3.0 renamed the key with no alias, and the chart's schema refuses the old one a 0.2 release still stores.
func renamedValues(stored, vals map[string]any, reset bool) []check.Result {
	old, held := valueAt(stored, constants.KeyApp, constants.KeySecurity, constants.KeyBreakGlass).(map[string]any)
	if reset || !held {
		return nil
	}
	// A null drops the key from the values Helm reuses.
	setValue(vals, nil, constants.KeyApp, constants.KeySecurity, constants.KeyBreakGlass)
	renamed := []string{constants.KeyApp, constants.KeyAuth, constants.KeyRequest, constants.KeyWaitHours}
	hours := old[constants.KeyWaitHours]
	if hours == nil || valueAt(vals, renamed...) != nil || valueAt(stored, renamed...) != nil {
		return []check.Result{check.Pass(constants.CheckRenamedValues, constants.MsgValueDropped)}
	}
	setValue(vals, hours, renamed...)
	return []check.Result{check.Pass(constants.CheckRenamedValues, fmt.Sprintf(constants.MsgValueMoved, hours))}
}

func setValue(vals map[string]any, value any, path ...string) {
	last := len(path) - constants.DefaultIncrementValue
	for _, key := range path[:last] {
		next, ok := vals[key].(map[string]any)
		if !ok {
			next = map[string]any{}
			vals[key] = next
		}
		vals = next
	}
	vals[path[last]] = value
}
