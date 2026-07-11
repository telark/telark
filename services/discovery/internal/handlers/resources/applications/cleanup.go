package applications

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/telark/discovery/internal/clients"
	"github.com/telark/discovery/internal/constants"
	redishelper "github.com/telark/discovery/internal/helpers/redis"
	sharedhelper "github.com/telark/discovery/internal/helpers/shared"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func CleanupApplicationData(w http.ResponseWriter, r *http.Request) {
	name, err := sharedhelper.GetPathParam(w, r, constants.NameParam)
	if err != nil {
		return
	}
	name = strings.TrimSpace(name)
	if name == constants.EmptyString {
		responseutils.LogAndSendResponse(
			w, http.StatusBadRequest, response.OperationError, string(constants.ErrAppNameRequired), nil, nil,
		)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), constants.AppCleanupHandlerTimeout)
	defer cancel()

	if forwarded := forwardCleanupToLeaderIfNeeded(ctx, w, r, name); forwarded {
		return
	}

	rdb := redishelper.NewRedisClient()
	if shouldSkipCleanup(ctx, rdb, name) {
		sendCleanupResponse(w, name)
		return
	}

	RunCleanup(ctx, rdb, name)
	sendCleanupResponse(w, name)
}

// RunCleanup performs the destructive cleanup steps (Redis state, snapshot
// directory, exporter CRD) for an application. Loop-guard and leader-forward
// are NOT performed here — callers (HTTP handler / detector loop) own gating.
// Safe to call from any leader-side context.
func RunCleanup(ctx context.Context, rdb *redis.Client, name string) {
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	lg.Info(fmt.Sprintf(string(constants.LogAppCleanupStarted), name))
	if rdb != nil {
		cleanupRedisByPatterns(ctx, rdb, name)
	}
	cleanupSnapshotDirectory(name)
	cleanupApplicationCRD(name)
	lg.Info(fmt.Sprintf(string(constants.LogAppCleanupDone), name))
}

func forwardCleanupToLeaderIfNeeded(ctx context.Context, w http.ResponseWriter, r *http.Request, name string) bool {
	coord, replicaID := getCoordinationBundle()
	if coord == nil || strings.TrimSpace(replicaID) == constants.EmptyString {
		return false
	}
	isLeader, err := coord.Election.IsLeader(ctx)
	if err != nil || isLeader {
		return false
	}
	leaderID, err := coord.Election.CurrentLeader(ctx)
	if err != nil {
		return false
	}
	leaderID = strings.TrimSpace(leaderID)
	if leaderID == constants.EmptyString || leaderID == replicaID {
		return false
	}
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	lg.Info(fmt.Sprintf(string(constants.LogAppCleanupForwarding), name, leaderID))
	if err := proxyCleanupRequestToLeader(ctx, w, r, name, leaderID); err != nil {
		responseutils.LogAndSendResponse(
			w, http.StatusBadGateway, response.OperationError, err.Error(), nil, err,
		)
	}
	return true
}

func proxyCleanupRequestToLeader(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	name string,
	leaderID string,
) error {
	targetURL, err := buildLeaderCleanupURL(leaderID, name)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, targetURL, nil)
	if err != nil {
		return fmt.Errorf(string(constants.ErrAppCleanupLeaderForwardFailed), leaderID, err)
	}
	req.Header = r.Header.Clone()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf(string(constants.ErrAppCleanupLeaderForwardFailed), leaderID, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf(string(constants.ErrAppCleanupLeaderForwardFailed), leaderID, readErr)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf(string(constants.ErrAppCleanupLeaderForwardStatus), resp.StatusCode, string(body))
	}

	maps.Copy(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)
	return nil
}

func buildLeaderCleanupURL(leaderID string, name string) (string, error) {
	rawPath := fmt.Sprintf(constants.CleanupProxyPathFormat, url.PathEscape(strings.TrimSpace(name)))
	u := &url.URL{
		Scheme: constants.HTTPScheme,
		Host:   net.JoinHostPort(strings.TrimSpace(leaderID), constants.MainPort),
		Path:   rawPath,
	}
	if strings.TrimSpace(u.Host) == constants.EmptyString {
		return constants.EmptyString, errors.New(string(constants.ErrForceSyncLeaderNotAvailable))
	}
	parsed, err := url.Parse(u.String())
	if err != nil {
		return constants.EmptyString, fmt.Errorf(string(constants.ErrAppCleanupLeaderForwardFailed), leaderID, err)
	}
	return parsed.String(), nil
}

func shouldSkipCleanup(ctx context.Context, rdb *redis.Client, name string) bool {
	if rdb == nil {
		return false
	}
	key := constants.KeyPrefixCleanupCooldown + name
	count, err := rdb.Incr(ctx, key).Result()
	if err != nil {
		return false
	}
	if count == constants.DefaultAddValue {
		_ = rdb.Expire(ctx, key, constants.CleanupCooldownTTL).Err()
		return false
	}
	if count == constants.CleanupLoopGuardLogAt {
		lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
		lg.Error(fmt.Sprintf(string(constants.LogAppCleanupLoopGuard), count, name))
	}
	return true
}

func sendCleanupResponse(w http.ResponseWriter, name string) {
	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		constants.ApplicationCleanupMessage,
		map[string]any{"name": name},
		nil,
	)
}

func cleanupRedisByPatterns(ctx context.Context, rdb *redis.Client, appName string) {
	patterns := []string{
		constants.ForceSyncStateKeyPrefix + appName + constants.Wildcard,
		constants.KeyPrefixLockApp + appName + constants.Wildcard,
		constants.KeyPrefixDedup + appName + constants.ColonSeparator + constants.Wildcard,
		constants.KeyPrefixGraceScale + appName + constants.Wildcard,
		constants.KeyPrefixIncidentState + appName + constants.Wildcard,
		constants.KeyPrefixOpState + appName + constants.ColonSeparator + constants.Wildcard,
	}
	for i := range patterns {
		deletePattern(ctx, rdb, patterns[i])
	}
}

func deletePattern(ctx context.Context, rdb *redis.Client, pattern string) {
	var cursor uint64
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	for {
		keys, nextCursor, err := rdb.Scan(ctx, cursor, pattern, constants.DefaultQueueSize).Result()
		if err != nil {
			lg.Error(fmt.Sprintf(string(constants.ErrAppCleanupRedisScanFailed), pattern, err))
			return
		}
		if len(keys) > constants.DefaultInitValue {
			if delErr := rdb.Del(ctx, keys...).Err(); delErr != nil {
				lg.Error(fmt.Sprintf(string(constants.ErrAppCleanupRedisDeleteFailed), pattern, delErr))
			}
		}
		cursor = nextCursor
		if cursor == uint64(constants.DefaultInitValue) {
			return
		}
	}
}

func cleanupSnapshotDirectory(appName string) {
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	basePath := filepath.Clean(filepath.Join(
		constants.DefaultSnapshotsBasePath,
		constants.DefaultSnapshotScopeDirectory,
	))
	appPath := filepath.Clean(filepath.Join(
		basePath,
		appName,
	))
	if !strings.HasPrefix(appPath, basePath+string(os.PathSeparator)) {
		lg.Error(fmt.Sprintf(string(constants.ErrAppCleanupSnapshotPathInvalid), appName))
		return
	}
	if err := os.RemoveAll(appPath); err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrAppCleanupSnapshotDeleteFailed), appName, err))
	}
}

func cleanupApplicationCRD(appName string) {
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	resp := clients.NewExporterClient().DeleteApplicationByName(appName)
	if resp == nil || (resp.Status != http.StatusOK && resp.Status != http.StatusNotFound) {
		lg.Error(fmt.Sprintf(string(constants.ErrAppCleanupExporterDeleteFailed), appName, resp))
	}
}
