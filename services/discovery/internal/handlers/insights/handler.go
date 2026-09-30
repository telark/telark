package insights

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"

	appresource "github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/insightsindex"
	"github.com/telark/telark/services/discovery/internal/discovery/cache"
	redishelper "github.com/telark/telark/services/discovery/internal/helpers/redis"
	tcfghelper "github.com/telark/telark/services/discovery/internal/helpers/telarkconfig"
)

const (
	appsParam    = "apps"
	appKeySep    = "/"
	appKeyFields = 2
	namespaceIdx = 0
	nameIdx      = 1
)

// Windowed to the apps the caller names, never the whole cluster, so the body stays bounded.
// Pending apps are still being analyzed and appear on a later poll.
type Response struct {
	Results map[string]*appresource.AppInsights `json:"results"`
	Pending []string                            `json:"pending"`
}

func GetApplicationsInsights(w http.ResponseWriter, r *http.Request) {
	keys := parseAppKeys(r.URL.Query().Get(appsParam))
	if len(keys) > constants.InsightsReadMaxApps {
		responseutils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationError,
			fmt.Sprintf(string(constants.ErrInsightsTooManyApps), constants.InsightsReadMaxApps), nil, nil)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), constants.InsightsReadTimeout)
	defer cancel()

	rdb := redishelper.NewRedisClient()
	out := Response{
		Results: make(map[string]*appresource.AppInsights, len(keys)),
		Pending: make([]string, constants.DefaultInitValue, len(keys)),
	}

	excluded := tcfghelper.FetchExcludedNamespaces(ctx)
	for _, key := range keys {
		namespace, name, ok := splitAppKey(key)
		if !ok || slices.Contains(excluded, namespace) {
			continue
		}
		insights, err := cache.GetInsights(ctx, rdb, namespace, name)
		if err != nil || insights == nil {
			out.Pending = append(out.Pending, key)
			continue
		}
		// Version stays the stored one: the UI compares it with stream events to decide a refetch.
		insights.Insights = slices.DeleteFunc(insights.Insights, func(card appresource.Insight) bool {
			return slices.Contains(excluded, insightsindex.WorkloadNamespace(&card, namespace))
		})
		out.Results[key] = insights
	}

	responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess,
		string(constants.InfoInsightsRead), out, nil)
}

func parseAppKeys(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == constants.EmptyString {
		return nil
	}
	parts := strings.Split(raw, ",")
	keys := make([]string, constants.DefaultInitValue, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != constants.EmptyString {
			keys = append(keys, trimmed)
		}
	}
	return keys
}

func splitAppKey(key string) (namespace, name string, ok bool) {
	parts := strings.SplitN(key, appKeySep, appKeyFields)
	if len(parts) != appKeyFields {
		return constants.EmptyString, constants.EmptyString, false
	}
	return parts[namespaceIdx], parts[nameIdx], true
}
