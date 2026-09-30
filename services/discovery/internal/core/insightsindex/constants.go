package insightsindex

import (
	"github.com/telark/telark/internal/data/plans"
	"github.com/telark/telark/internal/data/resources/application"
)

const (
	ParamCategory    = "category"
	ParamKind        = "kind"
	ParamSeverity    = "severity"
	ParamState       = "state"
	ParamTriage      = "triage"
	ParamNamespace   = "namespace"
	ParamEnvironment = "environment"
	ParamSearch      = "q"
	ParamPage        = "page"
	ParamPageSize    = "pageSize"
	ParamID          = "id"
	ParamApp         = "app"
	ParamFresh       = "fresh"

	StateStale = "stale"

	TriageUntriaged    = "untriaged"
	TriageAcknowledged = application.TriageStateAcknowledged
	TriageDismissed    = application.TriageStateDismissed
	TriageAll          = "all"

	DefaultPage     = 1
	DefaultPageSize = 25
	MaxPageSize     = 100

	listSeparator      = ","
	memberSeparator    = "/"
	cardParamNamespace = "namespace"
	// Keeps one field's text from matching across its neighbor in the search haystack.
	searchFieldSeparator = "\n"
	etagFieldSeparator   = "\x00"
	etagFormat           = `W/"%d-%d-%x"`
	scoreFormat          = 'f'
	scorePrecision       = -1
	scoreBits            = 64
	minScoreUnbounded    = "-inf"
	maxScoreUnbounded    = "+inf"
)

var (
	categories = []string{application.InsightCategoryIncident, application.InsightCategoryRecommendation}
	kinds      = []string{
		application.InsightKindCrashloop, application.InsightKindOOM, application.InsightKindImagePull,
		application.InsightKindProbeFailure, application.InsightKindScheduling, application.InsightKindRolloutStuck,
		application.InsightKindConfigChangeRegression, application.InsightKindResourcePressure, application.InsightKindOther,
		application.RecommendationKindReliability, application.RecommendationKindResources, application.RecommendationKindScaling,
		application.RecommendationKindSecurity, application.RecommendationKindImages, application.RecommendationKindConfig,
		application.RecommendationKindNetworking, application.RecommendationKindChangeRisk,
		application.RecommendationKindProtection, application.RecommendationKindConsistency,
	}
	severities = []string{application.InsightSeverityInfo, application.InsightSeverityWarning, application.InsightSeverityCritical}
	states     = []string{application.InsightStatusOpen, application.InsightStatusUpdated, application.InsightStatusResolved, StateStale}
	// The default view: everything still needing attention; sorted like a parsed list so both hash alike.
	defaultStates  = []string{application.InsightStatusOpen, StateStale, application.InsightStatusUpdated}
	triageFilters  = []string{TriageUntriaged, TriageAcknowledged, TriageDismissed, TriageAll}
	severityRank   = map[string]int{application.InsightSeverityInfo: 1, application.InsightSeverityWarning: 2, application.InsightSeverityCritical: 3}
	livePlanPhases = []string{plans.PhaseActive, plans.PhaseScheduled, plans.PhasePendingApproval, plans.PhaseDraft}
)
