package constants

// Input limits in characters (runes) and items, checked before every write; a refusal is a 400 naming the field.
const (
	PlanNamePattern      = `^[a-zA-Z0-9_\- ]+$`
	PlanParamItemMaxLen  = 256
	PlanParamItemsMax    = 50
	PlanParticipantsMax  = 50
	PlanScopeTargetsMax  = 200
	ReasonMaxLen         = 500
	InsightsSearchMaxLen = 200

	FieldPlanName             = "name"
	FieldPlanDescription      = "description"
	FieldParticipantRefs      = "participantRefs"
	FieldScopeNamespaces      = "scope.namespaces"
	FieldScopeApplicationRefs = "scope.applicationRefs"
	FieldReason               = "reason"
	FmtFieldPolicyParam       = "policies[%s].params.%s"
)
