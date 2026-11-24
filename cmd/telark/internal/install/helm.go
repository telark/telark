package install

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"slices"

	"github.com/telark/telark/cmd/telark/internal/constants"
	"github.com/telark/telark/cmd/telark/internal/kube"
	"github.com/telark/telark/cmd/telark/internal/output"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/cli/values"
	"helm.sh/helm/v4/pkg/getter"
	helmkube "helm.sh/helm/v4/pkg/kube"
	"helm.sh/helm/v4/pkg/release"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
	"helm.sh/helm/v4/pkg/storage/driver"
	"sigs.k8s.io/yaml"
)

// Helm's own logs would interleave with the check lines, so they're dropped and its errors reported instead.
func configuration(kubeFlags *kube.Flags) (*action.Configuration, error) {
	// helm and telark apply as one field manager, or each would conflict with the fields the other set.
	helmkube.ManagedFieldsManager = constants.HelmFieldManager
	cfg := action.NewConfiguration(action.ConfigurationSetLogger(slog.DiscardHandler))
	if err := cfg.Init(kubeFlags.RESTClientGetter(), kubeFlags.Namespace, constants.HelmDriver); err != nil {
		return nil, fmt.Errorf(constants.ErrHelmInit, err)
	}
	return cfg, nil
}

func merge(options *values.Options) (map[string]any, error) {
	return options.MergeValues(getter.Getters())
}

func waitStrategy(wait bool) helmkube.WaitStrategy {
	if wait {
		return helmkube.StatusWatcherStrategy
	}
	return helmkube.HookOnlyStrategy
}

func dryRunStrategy(dryRun bool) action.DryRunStrategy {
	if dryRun {
		return action.DryRunServer
	}
	return action.DryRunNone
}

// Same defaults as helm install and helm upgrade on Helm 4: server-side apply, a 5-minute timeout, 8 status workers.
func newInstall(cfg *action.Configuration, namespace string, wait, dryRun bool) *action.Install {
	install := action.NewInstall(cfg)
	install.ReleaseName, install.Namespace, install.CreateNamespace = constants.ReleaseName, namespace, true
	install.ServerSideApply, install.Timeout = true, constants.HelmTimeout
	install.WaitStrategy, install.DryRunStrategy = waitStrategy(wait), dryRunStrategy(dryRun)
	install.WaitOptions = []helmkube.WaitOption{helmkube.WithStatusComputeWorkers(constants.HelmStatusWorkers)}
	return install
}

func newUpgrade(cfg *action.Configuration, namespace string, reset, wait, dryRun bool) *action.Upgrade {
	upgrade := action.NewUpgrade(cfg)
	upgrade.Namespace, upgrade.ServerSideApply, upgrade.Timeout = namespace, constants.ServerSideApplyAuto, constants.HelmTimeout
	upgrade.ResetValues, upgrade.ResetThenReuseValues = reset, !reset
	upgrade.WaitStrategy, upgrade.DryRunStrategy = waitStrategy(wait), dryRunStrategy(dryRun)
	upgrade.WaitOptions = []helmkube.WaitOption{helmkube.WithStatusComputeWorkers(constants.HelmStatusWorkers)}
	return upgrade
}

// Helm 4's actions return the release behind an interface.
func accessor(released release.Releaser, err error) (release.Accessor, error) {
	if err != nil {
		return nil, err
	}
	return release.NewAccessor(released)
}

// Helm 4 returns its v1 release behind an interface, and nothing when the release doesn't exist.
func installedRelease(cfg *action.Configuration, namespace string) (*releasev1.Release, error) {
	current, err := action.NewGet(cfg).Run(constants.ReleaseName)
	if err != nil && !errors.Is(err, driver.ErrReleaseNotFound) {
		return nil, err
	}
	installed, ok := current.(*releasev1.Release)
	if !ok {
		return nil, fmt.Errorf(constants.ErrNoRelease, namespace)
	}
	return installed, nil
}

func valueAt(vals map[string]any, path ...string) any {
	var value any = vals
	for _, key := range path {
		level, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		value = level[key]
	}
	return value
}

func stringAt(vals map[string]any, path ...string) string {
	if text, ok := valueAt(vals, path...).(string); ok {
		return text
	}
	return constants.EmptyString
}

// The dry run shows what the install would create: the merged values and how many objects of each kind.
func writeDryRun(w io.Writer, manifest, chartVersion, namespace string, vals map[string]any) error {
	if _, err := fmt.Fprintf(w, constants.MsgDryRunInstall, chartVersion, namespace); err != nil {
		return err
	}
	if err := output.Write(w, output.FormatYAML, vals); err != nil {
		return err
	}
	if _, err := io.WriteString(w, constants.MsgDryRunObjects); err != nil {
		return err
	}
	kinds := map[string]int{}
	for _, document := range releaseutil.SplitManifests(manifest) {
		var head manifestHead
		if yaml.Unmarshal([]byte(document), &head) == nil && head.Kind != constants.EmptyString {
			kinds[head.Kind]++
		}
	}
	for _, kind := range slices.Sorted(maps.Keys(kinds)) {
		if _, err := fmt.Fprintf(w, constants.MsgObjectKind, kind, kinds[kind]); err != nil {
			return err
		}
	}
	return nil
}
