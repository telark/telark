package shared

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/telark/rest/base"
	"github.com/telark/rest/constants"
)

type debounceEntry struct {
	lastEmit   time.Time
	suppressed int64
}

var (
	durationLogMu      sync.Mutex
	durationLogTable   = map[string]*debounceEntry{}
	durationLogEnabled atomic.Bool
	durationLogDedup   atomic.Int64
	durationLogConfig  sync.Once
)

func loadDurationLogConfig() {
	enabled := true
	if raw := strings.TrimSpace(os.Getenv(constants.EnvExporterDurationLogEnabled)); raw != constants.EmptyString {
		if v, err := strconv.ParseBool(raw); err == nil {
			enabled = v
		}
	}
	durationLogEnabled.Store(enabled)

	dedup := int64(constants.DefaultExporterDurationLogDedupSec)
	if raw := strings.TrimSpace(os.Getenv(constants.EnvExporterDurationLogDedupSec)); raw != constants.EmptyString {
		if v, err := strconv.Atoi(raw); err == nil && v >= constants.EmptySliceLength {
			dedup = int64(v)
		}
	}
	durationLogDedup.Store(dedup)
}

func isExporterDurationLogActive(service base.Service) bool {
	durationLogConfig.Do(loadDurationLogConfig)
	if !durationLogEnabled.Load() {
		return false
	}
	return service == base.Exporter
}

func observeExporterCall(
	service base.Service,
	method base.Method,
	endpoint base.Endpoint,
	resp *http.Response,
	err error,
	elapsed time.Duration,
) {
	if !isExporterDurationLogActive(service) {
		return
	}
	statusCode := constants.NoStatusCode
	if resp != nil {
		statusCode = resp.StatusCode
	}
	elapsedMs := elapsed.Milliseconds()
	if err == nil && statusCode > constants.EmptySliceLength && statusCode < constants.HTTPErrorCode {
		return
	}
	safeEndpoint := string(sanitizeEndpoint(endpoint))
	emit, suppressed := shouldEmitError(string(service), string(method), safeEndpoint, statusCode)
	if !emit {
		return
	}
	line := fmt.Sprintf(
		string(constants.LogExporterCallError),
		string(service), string(method), safeEndpoint, statusCode, elapsedMs, suppressed,
		redactEndpointError(err, endpoint),
	)
	// A 4xx is the peer answering, not a fault: the caller already gets the
	// status back and decides whether it is an error.
	if err == nil && statusCode < constants.HTTPServerErrorCode {
		base.GetLogger().Debug(line)
		return
	}
	base.GetLogger().Warn(line)
}

func shouldEmitError(service, method, endpoint string, statusCode int) (bool, int64) {
	key := fmt.Sprintf("%s|%s|%s|%d", service, method, endpoint, statusCode)
	window := time.Duration(durationLogDedup.Load()) * time.Second
	now := time.Now()
	durationLogMu.Lock()
	defer durationLogMu.Unlock()
	entry, ok := durationLogTable[key]
	if !ok {
		durationLogTable[key] = &debounceEntry{lastEmit: now}
		return true, constants.EmptySliceLength
	}
	if window == constants.EmptySliceLength || now.Sub(entry.lastEmit) >= window {
		suppressed := entry.suppressed
		entry.lastEmit = now
		entry.suppressed = constants.EmptySliceLength
		return true, suppressed
	}
	entry.suppressed++
	return false, entry.suppressed
}
