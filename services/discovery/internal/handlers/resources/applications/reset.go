package applications

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/redis/go-redis/v9"
	dataconstants "github.com/telark/telark/internal/data/constants"
	insightsdata "github.com/telark/telark/internal/data/insights"
	kcorek8s "github.com/telark/telark/internal/kcore/k8sclient"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/discovery/internal/clients"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/coordination"
	redishelper "github.com/telark/telark/services/discovery/internal/helpers/redis"
	sharedhelper "github.com/telark/telark/services/discovery/internal/helpers/shared"
	tcfghelper "github.com/telark/telark/services/discovery/internal/helpers/telarkconfig"
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

	if app, getErr := clients.NewExporterClient().GetApplicationByNameFresh(name); getErr != nil || app == nil {
		responseutils.LogAndSendResponse(
			w, http.StatusNotFound, response.OperationNotFound, string(constants.MsgApplicationNotFound), nil, getErr,
		)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), constants.AppResetHandlerTimeout)
	defer cancel()

	rdb := redishelper.NewRedisClient()
	if forwarded := forwardResetToLeaderIfNeeded(ctx, rdb, w, r, name); forwarded {
		return
	}

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

// Loop-guard and leader-forward are NOT performed here — callers (HTTP handler / detector loop)
// own that gating. Safe to call from any leader-side context.
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

func forwardResetToLeaderIfNeeded(ctx context.Context, rdb *redis.Client, w http.ResponseWriter, r *http.Request, name string) bool {
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
	if err := proxyResetRequestToLeader(ctx, rdb, w, r, name, leaderID); err != nil {
		responseutils.LogAndSendResponse(
			w, http.StatusBadGateway, response.OperationError, err.Error(), nil, err,
		)
	}
	return true
}

var leaderForwardClient = &http.Client{
	Timeout: constants.AppResetForwardTimeout,
	// A redirect would carry the service token to wherever the answer points.
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func proxyResetRequestToLeader(
	ctx context.Context,
	rdb *redis.Client,
	w http.ResponseWriter,
	r *http.Request,
	name string,
	leaderID string,
) error {
	addr, err := verifiedLeaderAddress(ctx, rdb, leaderID)
	if err != nil {
		return fmt.Errorf(string(constants.ErrAppResetLeaderForwardFailed), leaderID, err)
	}
	req, err := BuildLeaderResetRequest(ctx, r, LeaderResetURL(addr, name))
	if err != nil {
		return fmt.Errorf(string(constants.ErrAppResetLeaderForwardFailed), leaderID, err)
	}

	resp, err := leaderForwardClient.Do(req)
	if err != nil {
		return fmt.Errorf(string(constants.ErrAppResetLeaderForwardFailed), leaderID, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, constants.MaxRequestBodyBytes))
	if readErr != nil {
		return fmt.Errorf(string(constants.ErrAppResetLeaderForwardFailed), leaderID, readErr)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf(string(constants.ErrAppResetLeaderForwardStatus), resp.StatusCode, string(body))
	}

	if contentType := resp.Header.Get(constants.HeaderContentType); contentType != constants.EmptyString {
		w.Header().Set(constants.HeaderContentType, contentType)
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)
	return nil
}

func verifiedLeaderAddress(ctx context.Context, rdb *redis.Client, leaderID string) (string, error) {
	namespace := tcfghelper.OwnNamespace()
	if namespace == constants.EmptyString {
		return constants.EmptyString, errors.New(string(constants.ErrLeaderNamespaceUnknown))
	}
	kube, err := kcorek8s.InitKubernetesClient()
	if err != nil {
		return constants.EmptyString, err
	}
	_, selfID := getCoordinationBundle()
	return coordination.VerifiedReplicaAddress(ctx, rdb, kube.CoreV1().Pods(namespace), selfID, leaderID)
}

// Never the inbound headers: the caller's session or service token must not travel to an address
// that came from Redis. The leader gets this replica's own service token and the verified caller.
func BuildLeaderResetRequest(ctx context.Context, r *http.Request, targetURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	if token := strings.TrimSpace(os.Getenv(dataconstants.EnvServiceToken)); token != constants.EmptyString {
		req.Header.Set(dataconstants.HeaderServiceToken, token)
	}
	if identity, ok := xauthz.FromContext(r.Context()); ok && identity.UserID != constants.EmptyString {
		req.Header.Set(constants.HeaderUserID, identity.UserID)
	}
	return req, nil
}

func LeaderResetURL(addr, name string) string {
	u := &url.URL{
		Scheme: constants.HTTPScheme,
		Host:   net.JoinHostPort(addr, constants.MainPort),
		Path:   fmt.Sprintf(constants.ResetProxyPathFormat, url.PathEscape(strings.TrimSpace(name))),
	}
	return u.String()
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

// shouldSkipReset arms the cooldown before the reset runs; left behind after a failure it turns
// the operator's retry into a 200 no-op. The handler context may be why the reset failed, so the
// delete does not share its deadline.
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
	// Exact keys or delimited prefixes only: `<app>*` also matched an app named `<app>-2`.
	patterns := []string{
		constants.ForceSyncStateKeyPrefix + appName,
		constants.KeyPrefixLockApp + appName,
		constants.KeyPrefixDedup + appName + constants.ColonSeparator + constants.Wildcard,
		constants.KeyPrefixGraceScale + appName,
		constants.KeyPrefixIncidentState + appName,
		constants.KeyPrefixOpState + appName + constants.ColonSeparator + constants.Wildcard,
		constants.KeyPrefixCoalesceBuffer + appName,
		constants.KeyPrefixCoalesceHeld + appName,
		constants.KeyPrefixHistoryRecorded + appName,
		constants.KeyPrefixHistoryPost + appName,
		constants.KeyPrefixHistoryFloor + appName,
		constants.KeyPrefixHistoryDeferred + appName,
		constants.KeyPrefixSnapshotPending + appName,
		constants.KeyPrefixLockGen + appName + constants.ColonSeparator + constants.Wildcard,
		constants.ForceSyncDedupKeyPrefix + appName,
		constants.KeyPrefixRollbackApplying + appName,
		constants.KeyPrefixLockRollback + appName,
		// The analyzer's document, cooldowns and run lease, in any namespace: the stored app may list none.
		insightsdata.DocumentKeyPrefix + constants.Wildcard + constants.ColonSeparator + appName,
	}
	for i := range patterns {
		deletePattern(ctx, rdb, patterns[i])
	}
	deleteAnalyzerEntries(ctx, rdb, appName)
}

// The analyzer's index member and usage and review fields are "<namespace>/<app>", for any namespace.
// They go after the document, the order the analyzer itself keeps.
func deleteAnalyzerEntries(ctx context.Context, rdb *redis.Client, appName string) {
	match := constants.Wildcard + constants.PathSeparator + appName
	for _, member := range scannedNames(ctx, rdb.ZScan, insightsdata.IndexKey, match) {
		logDeleteError(rdb.ZRem(ctx, insightsdata.IndexKey, member).Err(), insightsdata.IndexKey)
	}
	for _, key := range []string{constants.KeyAnalyzerUsage, constants.KeyAnalyzerReview} {
		for _, field := range scannedNames(ctx, rdb.HScan, key, match) {
			logDeleteError(rdb.HDel(ctx, key, field).Err(), key)
		}
	}
}

// ZSCAN and HSCAN answer name, value pairs: only the names are kept.
func scannedNames(
	ctx context.Context,
	scan func(context.Context, string, uint64, string, int64) *redis.ScanCmd,
	key, match string,
) []string {
	var out []string
	var cursor uint64
	for {
		pairs, next, err := scan(ctx, key, cursor, match, constants.DefaultQueueSize).Result()
		if err != nil {
			constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Error(
				fmt.Sprintf(string(constants.ErrAppResetRedisScanFailed), key, err))
			return out
		}
		for i := constants.DefaultInitValue; i < len(pairs); i += constants.TwoValue {
			out = append(out, pairs[i])
		}
		cursor = next
		if cursor == uint64(constants.DefaultInitValue) {
			return out
		}
	}
}

func logDeleteError(err error, key string) {
	if err != nil {
		constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Error(
			fmt.Sprintf(string(constants.ErrAppResetRedisDeleteFailed), key, err))
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
