package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/constants"
	"github.com/telark/discovery/discovery/shared"
	"github.com/redis/go-redis/v9"
)

func (ft *FlexTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	for _, layout := range DateTimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			ft.Time = t.UTC()
			return nil
		}
	}
	return fmt.Errorf(MsgCannotParseDatetime, s)
}

// NOTE: must match Python enrichment-service (enrichment:{namespace}:{name}).
func CacheKey(namespace, name string) string {
	return fmt.Sprintf("%s:%s:%s", cachePrefix, namespace, name)
}

func inflightKey(namespace, name string) string {
	return fmt.Sprintf("%s%s%s:%s", cachePrefix, inflightSuffix, namespace, name)
}

// reports whether a job for this app is already in-flight (Python worker).
func IsEnqueued(ctx context.Context, rdb *redis.Client, namespace, name string) bool {
	if rdb == nil {
		return false
	}
	key := inflightKey(namespace, name)
	result, err := rdb.Exists(ctx, key).Result()
	if err != nil {
		return false
	}
	return result > constants.DefaultInitValue
}

func GetEnrichment(ctx context.Context, rdb *redis.Client, namespace, name string) (*application.Insights, error) {
	if rdb == nil {
		constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(MsgRedisClientNil)
		return nil, nil
	}
	key := CacheKey(namespace, name)
	raw, err := rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(
			fmt.Sprintf(MsgCacheGetFailed, key, err))
		return nil, err
	}
	insights := parseCachedEnrichment(key, raw)
	if insights == nil {
		return nil, nil
	}
	return insights, nil
}

func parseCachedEnrichment(key, raw string) *application.Insights {
	var c cachedEnrichment
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(
			fmt.Sprintf(MsgCacheDeserializeFailed, key, err, raw))
		return nil
	}
	techStack := shared.CoalesceStrings(c.TechStack)
	if len(techStack) == constants.DefaultInitValue {
		techStack = shared.CoalesceStrings(c.TechStackSnake)
	}
	enrichedAt := c.EnrichedAt.Time
	if enrichedAt.IsZero() && c.EnrichedAtSnake != nil && *c.EnrichedAtSnake != constants.EmptyString {
		enrichedAt = parseEnrichedAtFallback(*c.EnrichedAtSnake)
	}
	var enrichedAtPtr *string
	if !enrichedAt.IsZero() {
		formatted := enrichedAt.Format(time.RFC3339)
		enrichedAtPtr = &formatted
	}
	var promptVersionPtr *string
	if c.PromptVersion != "" {
		promptVersionPtr = &c.PromptVersion
	}
	return &application.Insights{
		Enriched:      true,
		EnrichedAt:    enrichedAtPtr,
		Confidence:    ptrString(c.Confidence),
		Summary:       ptrString(c.Summary),
		TechStack:     techStack,
		Role:          ptrString(c.Role),
		Dependencies:  shared.CoalesceStrings(c.Dependencies),
		Category:      ptrString(c.Category),
		Risks:         shared.CoalesceStrings(c.Risks),
		Suggestions:   shared.CoalesceStrings(c.Suggestions),
		RelatedApps:   coalesceRelatedApps(c.RelatedApps),
		PromptVersion: promptVersionPtr,
	}
}

func coalesceRelatedApps(a []application.RelatedApp) []application.RelatedApp {
	if len(a) > constants.DefaultInitValue {
		return a
	}
	return []application.RelatedApp{}
}

func parseEnrichedAtFallback(s string) time.Time {
	for _, layout := range DateTimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func ptrString(s string) *string {
	if s == constants.EmptyString {
		return nil
	}
	return &s
}

func IsStale(insights *application.Insights, lastUpdated string) bool {
	if insights == nil {
		return true
	}
	t := parseLastUpdated(lastUpdated)
	if t.IsZero() {
		return true
	}
	enrichedTime := parseInsightEnrichedAt(insights.EnrichedAt)
	if !enrichedTime.IsZero() && enrichedTime.Before(t) {
		return true
	}
	return false
}

// parseInsightEnrichedAt parses application.Insights.EnrichedAt (*string RFC3339 or legacy layouts).
func parseInsightEnrichedAt(p *string) time.Time {
	if p == nil {
		return time.Time{}
	}
	s := strings.TrimSpace(*p)
	if s == constants.EmptyString {
		return time.Time{}
	}
	for _, layout := range DateTimeLayouts {
		if tm, err := time.Parse(layout, s); err == nil {
			return tm.UTC()
		}
	}
	return time.Time{}
}

func parseLastUpdated(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == constants.EmptyString {
		return time.Time{}
	}
	for _, layout := range DateTimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
