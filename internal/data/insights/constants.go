package insights

import "fmt"

const (
	StreamMaxLen            int64 = 1000
	StreamJobs                    = "insights:jobs"
	ConsumerGroup                 = "analyzer"
	FieldNamespace                = "namespace"
	FieldName                     = "name"
	FieldTrigger                  = "trigger"
	FieldGeneration               = "generation"
	DocumentKeyPrefix             = "analyzer:"
	IndexKey                      = "analyzer:index" // ZSET: member "<namespace>/<name>", score = last document write in unix ms.
	InflightKeyPrefix             = "analyzer:inflight:"
	CooldownManualKeyPrefix       = "analyzer:cooldown:manual:"
	CooldownAutoKeyPrefix         = "analyzer:cooldown:auto:"
	EventAnalysisQueued           = "analysis.queued"
	EventAnalysisStarted          = "analysis.started"
	EventAnalysisFailed           = "analysis.failed"
	EventAnalysisFinished         = "analysis.finished"
	EventInsightCreated           = "insight.created"
	EventInsightUpdated           = "insight.updated"
	EventInsightResolved          = "insight.resolved"
	EventRuntimeChanged           = "runtime.changed"
	EventRuntimePull              = "runtime.pull"
	EventResync                   = "resync"
	// Must stay identical to spec.ai.model's pattern in the TelarkConfig CRD.
	ModelNamePattern = `^[a-z0-9][a-z0-9._-]*(:[a-z0-9._-]+)?$`
)

func DocumentKey(namespace, name string) string {
	return fmt.Sprintf("%s%s:%s", DocumentKeyPrefix, namespace, name)
}
