package install

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/telark/telark/cmd/telark/internal/check"
	"github.com/telark/telark/cmd/telark/internal/constants"
	"helm.sh/helm/v4/pkg/action"
	ci "helm.sh/helm/v4/pkg/chart"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	"helm.sh/helm/v4/pkg/registry"
)

func newRegistry() (*registry.Client, error) {
	return registry.NewClient(registry.ClientOptEnableCache(true), registry.ClientOptWriter(io.Discard))
}

// A registry chart is verified by the digest it was pulled at, so what runs is what was signed.
func (s source) chart(ctx context.Context) (*chartv2.Chart, check.Result, error) {
	if s.directory != constants.EmptyString {
		chart, err := loader.Load(s.directory)
		if err != nil {
			return nil, check.Result{}, fmt.Errorf(constants.ErrLoadChart, s.directory, err)
		}
		return chart, check.Warn(constants.CheckSignature, fmt.Sprintf(constants.MsgSignatureSkipped, s.directory)), dependencies(chart, s.directory)
	}
	ref := constants.ChartRepository + constants.ChartTagSeparator + s.version
	pulled, err := s.registry.Pull(ref)
	if err != nil {
		return nil, check.Result{}, fmt.Errorf(constants.ErrPullChart, ref, err)
	}
	chart, err := loader.LoadArchive(bytes.NewReader(pulled.Chart.Data))
	if err != nil {
		return nil, check.Result{}, fmt.Errorf(constants.ErrLoadChart, ref, err)
	}
	return chart, verify(ctx, constants.ChartRepository+constants.ChartDigestSeparator+pulled.Manifest.Digest, chart.Metadata.Version), nil
}

// A checkout without make deps lacks its subcharts, and installing it would delete theirs from the release.
func dependencies(chart *chartv2.Chart, directory string) error {
	required := make([]ci.Dependency, len(chart.Metadata.Dependencies))
	for i, dependency := range chart.Metadata.Dependencies {
		required[i] = dependency
	}
	if err := action.CheckDependencies(chart, required); err != nil {
		return fmt.Errorf(constants.ErrChartDependencies, directory, err)
	}
	return nil
}

// The release workflow signs each chart keyless from the run on its version's tag, so only that run's signature counts.
// sigstore-go would add 19 MB to the CLI, so cosign checks it.
func verify(ctx context.Context, ref, version string) check.Result {
	if _, err := exec.LookPath(constants.CosignCommand); err != nil {
		return check.Warn(constants.CheckSignature, constants.MsgSignatureNoCosign)
	}
	var stderr bytes.Buffer
	command := exec.CommandContext(ctx, constants.CosignCommand, constants.CosignVerify, ref,
		constants.CosignIdentityFlag, fmt.Sprintf(constants.CosignIdentity, version), constants.CosignIssuerFlag, constants.CosignIssuer)
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return check.Fail(constants.CheckSignature, fmt.Sprintf(constants.MsgSignatureInvalid, cmp.Or(lastLine(stderr.String()), err.Error())))
	}
	return check.Pass(constants.CheckSignature, fmt.Sprintf(constants.MsgSignatureValid, ref))
}

func lastLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), constants.NewLine)
	return lines[len(lines)-constants.DefaultIncrementValue]
}
