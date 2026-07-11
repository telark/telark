package shared

const (
	ChangeSourceHelm      = "helm"
	ChangeSourceKubectl   = "kubectl"
	SeverityCritical      = "critical"
	SeverityHigh          = "high"
	SeverityMedium        = "medium"
	SeverityLow           = "low"
	DriftCategoryMinCount = 2
	SourceHPA             = "hpa"
	SourceOperator        = "operator"
	SourceUnknown         = "unknown"
	ManagedByUnknown      = "unknown"
	FingerprintSep        = ":"
	FingerprintJoinSep    = "|"
	ManifestStatusKey     = "status"
	PayloadKeyKind        = "kind"
	PayloadKeyName        = "name"
	PayloadKeyNamespace   = "namespace"
	PayloadKeyManifest    = "manifest"
	PayloadKeyResources   = "resources"
	PayloadKeyNote        = "note"
)
