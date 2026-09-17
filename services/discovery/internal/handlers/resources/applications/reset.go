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
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/telark/discovery/internal/clients"
	"github.com/telark/discovery/internal/constants"
	redishelper "github.com/telark/discovery/internal/helpers/redis"
	sharedhelper "github.com/telark/discovery/internal/helpers/shared"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func ResetApplication(w http.ResponseWriter, r *http.Request) {
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

	ctx, cancel := context.WithTimeout(context.Background(), constants.AppResetHandlerTimeout)
	defer cancel()

	if forwarded := forwardResetToLeaderIfNeeded(ctx, w, r, name); forwarded {
		return
	}

	rdb := redishelper.NewRedisClient()
	if shouldSkipReset(ctx, rdb, name) {
		sendResetResponse(w, name)
		return
	}

	if err := RunReset(ctx, rdb, name); err != nil {
		clearResetCooldown(ctx, rdb, name)
		responseutils.LogAndSendResponse(w, http.StatusBadGateway, response.OperationError, err.Error(), nil, err)
		return
	}
	sendResetResponse(w, name)
}

// RunReset performs the destructive reset steps (Redis state, exporter CRD;
// the exporter removes the CRD's snapshot files) for an application. Loop-guard and leader-forward
// are NOT performed here — callers (HTTP handler / detector loop) own gating.
// Safe to call from any leader-side context.
func RunReset(ctx context.Context, rdb *redis.Client, name string) error {
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	lg.Info(fmt.Sprintf(string(constants.LogAppResetStarted), name))
	if rdb != nil {
		deleteRedisByPatterns(ctx, rdb, name)
	}
	if err := deleteApplicationCRD(name); err != nil {
		return err
	}
	lg.Info(fmt.Sprintf(string(constants.LogAppResetDone), name))
	return nil
}

func forwardResetToLeaderIfNeeded(ctx context.Context, w http.ResponseWriter, r *http.Request, name string) bool {
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
	lg.Info(fmt.Sprintf(string(constants.LogAppResetForwarding), name, leaderID))
	if err := proxyResetRequestToLeader(ctx, w, r, name, leaderID); err != nil {
		responseutils.LogAndSendResponse(
			w, http.StatusBadGateway, response.OperationError, err.Error(), nil, err,
		)
	}
	return true
}

func proxyResetRequestToLeader(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	name string,
	leaderID string,
) error {
	targetURL, err := buildLeaderResetURL(leaderID, name)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, nil)
	if err != nil {
		return fmt.Errorf(string(constants.ErrAppResetLeaderForwardFailed), leaderID, err)
	}
	req.Header = r.Header.Clone()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf(string(constants.ErrAppResetLeaderForwardFailed), leaderID, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf(string(constants.ErrAppResetLeaderForwardFailed), leaderID, readErr)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf(string(constants.ErrAppResetLeaderForwardStatus), resp.StatusCode, string(body))
	}

	maps.Copy(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)
	return nil
}

func buildLeaderResetURL(leaderID string, name string) (string, error) {
	rawPath := fmt.Sprintf(constants.ResetProxyPathFormat, url.PathEscape(strings.TrimSpace(name)))
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
		return constants.EmptyString, fmt.Errorf(string(constants.ErrAppResetLeaderForwardFailed), leaderID, err)
	}
	return parsed.String(), nil
}

func shouldSkipReset(ctx context.Context, rdb *redis.Client, name string) bool {
	if rdb == nil {
		return false
	}
	key := constants.KeyPrefixResetCooldown + name
	count, err := rdb.Incr(ctx, key).Result()
	if err != nil {
		return false
	}
	if count == constants.DefaultAddValue {
		_ = rdb.Expire(ctx, key, constants.ResetCooldownTTL).Err()
		return false
	}
	if count == constants.ResetLoopGuardLogAt {
		lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
		lg.Error(fmt.Sprintf(string(constants.LogAppResetLoopGuard), count, name))
	}
	return true
}

// shouldSkipReset arms the cooldown before the reset runs; left behind after a
// failure it turns the operator's retry into a 200 no-op. The handler context
// may be why the reset failed, so the delete does not share its deadline.
func clearResetCooldown(ctx context.Context, rdb *redis.Client, name string) {
	if rdb == nil {
		return
	}
	_ = rdb.Del(context.WithoutCancel(ctx), constants.KeyPrefixResetCooldown+name).Err()
}

func sendResetResponse(w http.ResponseWriter, name string) {
	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		constants.ApplicationResetMessage,
		map[string]any{"name": name},
		nil,
	)
}

func deleteRedisByPatterns(ctx context.Context, rdb *redis.Client, appName string) {
	patterns := []string{
		constants.ForceSyncStateKeyPrefix + appName + constants.Wildcard,
		constants.KeyPrefixLockApp + appName + constants.Wildcard,
		constants.KeyPrefixDedup + appName + constants.ColonSeparator + constants.Wildcard,
		constants.KeyPrefixGraceScale + appName + constants.Wildcard,
		constants.KeyPrefixIncidentState + appName + constants.Wildcard,
		constants.KeyPrefixOpState + appName + constants.ColonSeparator + constants.Wildcard,
		constants.KeyPrefixCoalesceBuffer + appName,
		constants.KeyPrefixLockGen + appName + constants.ColonSeparator + constants.Wildcard,
		constants.ForceSyncDedupKeyPrefix + appName,
		constants.KeyPrefixRollbackApplying + appName,
		constants.KeyPrefixLockRollback + appName,
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
			lg.Error(fmt.Sprintf(string(constants.ErrAppResetRedisScanFailed), pattern, err))
			return
		}
		if len(keys) > constants.DefaultInitValue {
			if delErr := rdb.Del(ctx, keys...).Err(); delErr != nil {
				lg.Error(fmt.Sprintf(string(constants.ErrAppResetRedisDeleteFailed), pattern, delErr))
			}
		}
		cursor = nextCursor
		if cursor == uint64(constants.DefaultInitValue) {
			return
		}
	}
}

func deleteApplicationCRD(appName string) error {
	resp := clients.NewExporterClient().DeleteApplicationByName(appName)
	if resp == nil || (resp.Status != http.StatusOK && resp.Status != http.StatusNotFound) {
		return fmt.Errorf(string(constants.ErrAppResetExporterDeleteFailed), appName, resp)
	}
	return nil
}
