package insights

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/insightsindex"
	redishelper "github.com/telark/discovery/internal/helpers/redis"
	tcfghelper "github.com/telark/discovery/internal/helpers/telarkconfig"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

const (
	etagListSeparator = ","
	etagWildcard      = "*"
)

var index atomic.Pointer[insightsindex.Index]

func InitIndex(idx *insightsindex.Index) {
	index.Store(idx)
}

// Served from this replica's row index; Redis is read on the request path only for `fresh` reads.
func ListInsights(w http.ResponseWriter, r *http.Request) {
	arrived := time.Now()
	q, err := insightsindex.ParseQuery(r.URL.Query())
	if err != nil {
		responseutils.SendResponse(w, http.StatusBadRequest, response.OperationError, err.Error(), nil)
		return
	}
	idx := index.Load()
	if idx == nil || !idx.Loaded() {
		notReady(w)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), constants.InsightsReadTimeout)
	defer cancel()
	// Fails closed: a cluster-wide list must never leak the namespaces the product hides.
	excluded, err := tcfghelper.ExcludedNamespaces(ctx)
	if err != nil {
		notReady(w)
		return
	}
	if q.Fresh {
		catchUp(ctx, idx, arrived)
	}
	now := time.Now()
	if etag := idx.ETag(&q, excluded, now); matchesETag(r.Header.Get(constants.HeaderIfNoneMatch), etag) {
		w.Header().Set(constants.HeaderETag, etag)
		w.WriteHeader(http.StatusNotModified)
		return
	}
	page, etag := idx.Query(&q, excluded, now)
	w.Header().Set(constants.HeaderETag, etag)
	responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess,
		string(constants.InfoInsightsListed), page, nil)
}

// Best effort: without Redis the reader gets the index as it is, which the next refresh corrects.
func catchUp(ctx context.Context, idx *insightsindex.Index, arrived time.Time) {
	rdb := redishelper.NewRedisClient()
	if rdb == nil {
		return
	}
	if err := idx.CatchUp(ctx, rdb, arrived); err != nil {
		constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(
			fmt.Sprintf(string(constants.WarnInsightsIndexCatchUpFailed), err),
		)
	}
}

func notReady(w http.ResponseWriter) {
	w.Header().Set(constants.HeaderRetryAfter, strconv.Itoa(constants.InsightsIndexRetryAfterSec))
	responseutils.SendResponse(w, http.StatusServiceUnavailable, response.OperationError,
		string(constants.InfoInsightsListNotReady), nil)
}

func matchesETag(header, etag string) bool {
	for candidate := range strings.SplitSeq(header, etagListSeparator) {
		if trimmed := strings.TrimSpace(candidate); trimmed == etag || trimmed == etagWildcard {
			return true
		}
	}
	return false
}
