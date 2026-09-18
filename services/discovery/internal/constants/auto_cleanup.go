package constants

import "time"

const (
	EnvAutoCleanupEnabled             = "DISCOVERY_AUTO_CLEANUP_ENABLED"
	EnvAutoCleanupDeleteEnabled       = "DISCOVERY_AUTO_CLEANUP_DELETE_ENABLED"
	EnvAutoCleanupCycleIntervalSec    = "DISCOVERY_AUTO_CLEANUP_CYCLE_INTERVAL_SEC"
	EnvAutoCleanupEmptyCyclesRequired = "DISCOVERY_AUTO_CLEANUP_EMPTY_CYCLES_REQUIRED"
	EnvAutoCleanupGracePeriodSec      = "DISCOVERY_AUTO_CLEANUP_GRACE_PERIOD_SEC"
)

const (
	DefaultAutoCleanupEnabled             = false
	DefaultAutoCleanupDeleteEnabled       = false
	DefaultAutoCleanupCycleInterval       = 5 * time.Minute
	DefaultAutoCleanupEmptyCyclesRequired = 3
	DefaultAutoCleanupGracePeriod         = 15 * time.Minute
)

const (
	KeyPrefixAutoCleanupEmptyStreak = "cleanup:empty_streak:"
	KeyPrefixAutoCleanupInflight    = "cleanup:auto:inflight:"
)

const (
	RailResourcesEmpty    = "resources_empty"
	RailNamespaceExists   = "namespace_exists"
	RailNamespaceIncluded = "namespace_included"
	RailNoActiveRollback  = "no_active_rollback"
	RailNoForceSync       = "no_force_sync"
	RailNoCoalesceBuffer  = "no_coalesce_buffer"
	RailNoGenerationLock  = "no_generation_lock"
	RailNoEnrichmentLock  = "no_enrichment_lock"
	RailSustainedAbsence  = "sustained_absence"
	RailGracePeriod       = "grace_period"
	RailCleanupCooldown   = "cleanup_cooldown"
)

const (
	AutoCleanupInflightTTLBuffer = 30 * time.Second
	AutoCleanupRailReadTimeout   = 5 * time.Second
)
