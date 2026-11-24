package check

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/telark/telark/cmd/telark/internal/constants"
	utilversion "k8s.io/apimachinery/pkg/util/version"
)

// One GET from the operator's machine, never from the cluster, and any failure only skips the check.
func latestRelease(ctx context.Context, current string) Result {
	ctx, cancel := context.WithTimeout(ctx, constants.UpdateCheckTimeout)
	defer cancel()
	// The method and the URL are constants, so the request always builds.
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, constants.ReleasesURL, http.NoBody)
	request.Header.Set(constants.HeaderAccept, constants.GitHubJSON)
	request.Header.Set(constants.HeaderUserAgent, fmt.Sprintf(constants.UserAgentFormat, current))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return skipped(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return skipped(fmt.Errorf(constants.ErrReleasesStatus, response.Status))
	}
	var latest release
	if err := json.NewDecoder(io.LimitReader(response.Body, constants.MaxReleaseBytes)).Decode(&latest); err != nil {
		return skipped(err)
	}
	newest, err := utilversion.ParseGeneric(latest.TagName)
	if err != nil {
		return skipped(err)
	}
	if running, err := utilversion.ParseGeneric(current); err == nil && !running.LessThan(newest) {
		return Pass(constants.CheckUpdate, fmt.Sprintf(constants.MsgUpToDate, current))
	}
	tag := strings.TrimPrefix(latest.TagName, constants.TagPrefix)
	return Warn(constants.CheckUpdate, fmt.Sprintf(constants.MsgUpdateAvailable, tag, current, latest.HTMLURL))
}

func skipped(err error) Result {
	return Warn(constants.CheckUpdate, fmt.Sprintf(constants.MsgUpdateSkipped, err))
}
