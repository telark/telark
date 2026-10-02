package constants

import (
	"time"

	reportseps "github.com/telark/telark/internal/rest/endpoints/reports"
)

const (
	OpCreate                = "create"
	OpGet                   = "get"
	OpList                  = "list"
	OpUpdate                = "update"
	OpPatch                 = "patch"
	OpDelete                = "delete"
	OpSync                  = "sync"
	ResourceUser            = "users"
	ResourceGroup           = "groups"
	ResourceCategory        = "categories"
	ResourceRole            = "accessroles"
	ResourceApplication     = "applications"
	ResourceUserSession     = "user-sessions"
	ResourceUserPasskey     = "user-passkeys"
	ResourceSnapshot        = "snapshots"
	CachedResponse          = "Cached response"
	HeaderETag              = "ETag"
	HeaderIfNoneMatch       = "If-None-Match"
	WeakETagPrefix          = "W/"
	NameParam               = "name"
	IDParam                 = "id"
	TypeParam               = "type"
	ScopeParam              = "scope"
	NamespaceParam          = "namespace"
	GenerationParam         = "generation"
	ViewParam               = "view"
	ViewSummary             = "summary"
	ViewFull                = "full"
	UsernameParam           = "username"
	EmailParam              = "email"
	UserIDParam             = "userId"
	CredentialIDParam       = "credentialId"
	IDsParam                = "ids"
	UserIDsSeparator        = ","
	UserNamesMaxIDs         = 100
	SpecField               = "spec"
	MetadataField           = "metadata"
	ResourceVersionField    = "resourceVersion"
	ConflictStatus          = 409
	ConflictMessageFragment = "the object has been modified"
	AllowedEnvVarPattern    = `^[A-Z_][A-Z0-9_]*$`
	MaxEnvVarLength         = 8192
	CertNotBeforeClockSkew  = 5 * time.Minute
	CacheKeyPrefix          = "cache"
	CacheGenerationSegment  = "generation"
	CacheTTL                = 5 * time.Minute
	// Single-resource copies: CRs are also patched directly by the discovery
	// rollback controller, so a cached copy must not outlive a short window.
	GetCacheTTL = 15 * time.Second
	// Lists may lag writes by this much; in exchange a write storm costs one
	// list rebuild per window instead of one per write.
	ListCacheMinAge   = 3 * time.Second
	ListCacheDirtyTTL = time.Minute
	// Rendered list blobs kept in process, newest last; the key carries the
	// generation, so an entry stops being read the moment the list changes.
	ListBlobLocalEntries     = 4
	ListRenderConcurrencyEnv = "EXPORTER_LIST_RENDER_CONCURRENCY"
	BootstrapAdminEnv        = "BOOTSTRAP_ADMIN"
	// A full list is megabytes of marshal buffer; beyond this many renders at
	// once a request queues for a slot until its own deadline.
	DefaultListRenderConcurrency                = 2
	MinListRenderConcurrency                    = 1
	ListRenderRetryAfter                        = "2"
	HeaderRetryAfter                            = "Retry-After"
	HeaderContentLength                         = "Content-Length"
	ResponseDataField                           = "data"
	CachedEnvelopeSentinel                      = `"__blob__"`
	InformerNoResync                            = 0
	InformerSyncTimeout                         = 30 * time.Second
	InformerKeyFormat                           = "%s/%s"
	ReadinessPingTimeout                        = time.Second
	ReadinessReasonRedis                        = "redis unreachable"
	ReadinessReasonInformer                     = "application informer not synced"
	ReadinessReasonKubernetes                   = "kubernetes api unreachable"
	PrefixInformers                             = "Informers: "
	CacheWindowSegment                          = "window"
	CacheDirtySegment                           = "dirty"
	CacheRestrictedSegment                      = "restricted"
	CacheKeySeparator                           = ":"
	DefaultPort                                 = 8080
	EmptyString                                 = ""
	CacheScanCount                       int64  = 100
	CacheScanCursorEnd                   uint64 = 0
	IndexLastElementOffset                      = 1
	DefaultChannelBufferSize                    = 1
	DefaultLastPasskeyCount                     = 1
	DefaultInitValue                            = 0
	DefaultIncrementValue                       = 1
	DefaultMajorPart                            = 2
	MinCategoriesInCRD                          = 1
	ServerReadTimeout                           = 30 * time.Second
	ServerWriteTimeout                          = 30 * time.Second
	ServerShutdownTimeout                       = 30 * time.Second
	ServerIdleTimeout                           = 120 * time.Second
	PrefixCache                                 = "Cache: "
	PrefixGenerics                              = "Generics: "
	PrefixOptimizer                             = "Optimizer: "
	PrefixMain                                  = "Main: "
	PrefixStartup                               = "Startup: "
	PrefixShared                                = "Shared: "
	LogMessageWithError                         = "%s: %v"
	MaxHeaderBytes                              = 1 << 20
	DefaultRoutesCount                          = 35
	SnapshotBytesPerKilobyte                    = 1024
	SnapshotKilobytesPerMegabyte                = 1024
	SnapshotPercentScale                        = 100
	TelarkConfigResourceName                    = "default"
	ManifestOrderServiceAccount                 = 1
	ManifestOrderConfigMap                      = 2
	ManifestOrderSecret                         = 3
	ManifestOrderPersistentVolumeClaim          = 4
	ManifestOrderService                        = 5
	ManifestOrderNetworkPolicy                  = 6
	ManifestOrderDeployment                     = 7
	ManifestOrderStatefulSet                    = 8
	ManifestOrderDaemonSet                      = 9
	ManifestOrderJob                            = 10
	ManifestOrderCronJob                        = 11
	ManifestOrderIngress                        = 12
	ManifestOrderHorizontalPodAutoscaler        = 13
	ManifestOrderVerticalPodAutoscaler          = 14
	DefaultManifestUnknownOrder                 = 99
	MaxUserIDGenerationAttempts                 = 10
	ResourceTypePasskey                         = "passkey"
	FieldCredentialID                           = "credentialId"
	FieldUsername                               = "username"
	FieldFullname                               = "fullname"
	FieldEmail                                  = "email"
	FieldIdentities                             = "identities"
	FieldProvider                               = "provider"
	FieldIssuer                                 = "issuer"
	FieldSubject                                = "subject"
	FieldAvatar                                 = "avatar"
	FieldSettings                               = "settings"
	FieldStatus                                 = "status"
	FieldPhase                                  = "phase"
	FieldInvite                                 = "invite"
	FieldInviteAcceptedAt                       = "inviteAcceptedAt"
	FieldID                                     = "id"
	FieldGeneration                             = "generation"
	FieldName                                   = "name"
	FieldItems                                  = "items"
	FieldScope                                  = "scope"
	FieldNamespace                              = "namespace"
	FieldType                                   = "type"
	FieldCreationDate                           = "creationDate"
	FieldCategories                             = "categories"
	FieldManifest                               = "manifest"
	FieldTotalPVCSpace                          = "totalPVCSpace"
	FieldConsumedSpace                          = "consumedSpace"
	FieldAvailableSpace                         = "availableSpace"
	FieldTotalSnapshots                         = "totalSnapshots"
	FieldUpdatedAt                              = "updatedAt"
	FieldSnapshotsPath                          = "snapshotsPath"
	FieldSnapshotScopes                         = "snapshotScopes"
	FieldPVCName                                = "pvcName"
	FieldPVCNamespace                           = "pvcNamespace"
	FieldBytes                                  = "bytes"
	FieldKilobytes                              = "kb"
	FieldMegabytes                              = "mb"
	FieldPercent                                = "percent"
	FieldResources                              = "resources"
	FieldSnapshots                              = "snapshots"
	FieldRollbacks                              = "rollbacks"
	FieldMetrics                                = "metrics"
	FieldWorkloads                              = "workloads"
	FieldHistory                                = "history"
	FieldChangeLog                              = "changeLog"
	FieldChanges                                = "changes"
	FieldPath                                   = "path"
	FieldAPIVersion                             = "apiVersion"
	FieldKind                                   = "kind"
	CategoriesCRDName                           = "categories"
	FieldFinalizers                             = "finalizers"
	FieldPriority                               = "priority"
	FieldVersion                                = "version"
	FieldValidity                               = "validity"
	FieldAutoRevoke                             = "autoRevoke"
	FieldExpiresTimestamp                       = "expiresTimestamp"
	FieldUserRefs                               = "userRefs"
	FieldRoleRefs                               = "roleRefs"
	FieldGroupRefs                              = "groupRefs"
	FieldDeletionTimestamp                      = "deletionTimestamp"
	FieldBootstrap                              = "bootstrap"
	FieldDisplayName                            = "displayName"
	FieldDescription                            = "description"
	FieldScopesAndPermissions                   = "scopesAndPermissions"
	MaxApplicationDisplayNameLength             = 200
	MaxApplicationDescriptionLength             = 1000
	ListSeparator                               = ", "
	NilFieldPath                                = "<nil>"
	HeaderUserID                                = "X-User-ID"
	SnapshotsPathEnv                            = "SNAPSHOTS_PATH"
	SnapshotsMaxVersionsEnv                     = "SNAPSHOTS_MAX_VERSIONS"
	SnapshotsPVCNameEnv                         = "SNAPSHOTS_PVC_NAME"
	SnapshotsPVCNamespaceEnv                    = "SNAPSHOTS_PVC_NAMESPACE"
	SnapshotGCIntervalSecEnv                    = "SNAPSHOT_GC_INTERVAL_SEC"
	DefaultSnapshotGCInterval                   = time.Hour
	SnapshotGCMinAge                            = time.Hour
	SnapshotGCLockKey                           = "exporter:snapshot:gc"
	SnapshotGCLockTTLDivisor                    = 2
	SnapshotStatsRefreshSecEnv                  = "SNAPSHOT_STATS_REFRESH_SEC"
	DefaultSnapshotStatsRefreshInterval         = time.Minute
	SnapshotTempFileSuffix                      = ".tmp"
	SnapshotsAppsSubdir                         = "apps"
	DefaultSnapshotsPath                        = "/snapshots"
	DefaultSnapshotsMaxVersions                 = 5
	MinSnapshotsMaxVersions                     = 1
	DefaultSnapshotsPVCName                     = "telark-exporter-snapshots-pvc"
	DefaultSnapshotsPVCNamespace                = "telark"
	SnapshotFileExtension                       = ".json"
	SnapshotFilePrefix                          = "V"
	SnapshotFileNameTemplate                    = SnapshotFilePrefix + "%d" + SnapshotFileExtension
	SnapshotRollbackFilenameSuffix              = "-rollback.json"
	SnapshotRollbackYAMLFilenameSuffix          = "-rollback.yaml"
	HeaderContentDisposition                    = "Content-Disposition"
	HeaderContentType                           = "Content-Type"
	ContentTypeJSON                             = "application/json"
	ContentDispositionAttachmentTemplate        = "attachment; filename=\"%s\""
	SnapshotLatestGenerationValue               = "latest"
	SnapshotDirPerm                             = 0o755
	// The star is where the unique part goes; the suffix keeps orphaned temp
	// files greppable.
	SnapshotTempSuffix             = ".*.tmp"
	SnapshotGenerationMinValue     = 1
	ReportsPathEnv                 = "REPORTS_PATH"
	DefaultReportsPath             = "/reports"
	ReportsPlansSubdir             = "plans"
	ReportsReportsSubdir           = "reports"
	ReportsLedgerFile              = "ledger.json"
	ReportsMaxPerPlan              = 10
	ReportsListDefaultLimit        = 200
	ReportsListMaxLimit            = 1000
	ReportsListSeparator           = ","
	ReportFileExtension            = ".json"
	ReportMaxBodyBytes             = 32 << 20
	ReportsSweepMinAge             = time.Hour
	ReportsGCLockKey               = "exporter:reports:gc"
	HeaderContentTypeOptions       = "X-Content-Type-Options"
	ContentTypeOptionsNoSniff      = "nosniff"
	HeaderCSP                      = "Content-Security-Policy"
	CSPSandbox                     = "sandbox"
	ContentTypeHTML                = "text/html; charset=utf-8"
	ContentTypeMarkdown            = "text/markdown; charset=utf-8"
	ContentTypeCSV                 = "text/csv; charset=utf-8"
	KindServiceAccount             = "ServiceAccount"
	KindConfigMap                  = "ConfigMap"
	KindSecret                     = "Secret"
	SecretValueRedacted            = "[redacted]"
	KindPersistentVolumeClaim      = "PersistentVolumeClaim"
	KindService                    = "Service"
	KindNetworkPolicy              = "NetworkPolicy"
	KindDeployment                 = "Deployment"
	KindStatefulSet                = "StatefulSet"
	KindDaemonSet                  = "DaemonSet"
	KindJob                        = "Job"
	KindCronJob                    = "CronJob"
	KindIngress                    = "Ingress"
	KindHorizontalPodAutoscaler    = "HorizontalPodAutoscaler"
	KindVerticalPodAutoscaler      = "VerticalPodAutoscaler"
	PasskeyDeviceTypePlatform      = "platform"
	PasskeyDeviceTypeCrossPlatform = "cross-platform"
	UnknownValue                   = "unknown"
)

var ReportContentTypes = map[string]string{
	reportseps.FormatHTML:     ContentTypeHTML,
	reportseps.FormatMarkdown: ContentTypeMarkdown,
	reportseps.FormatJSON:     ContentTypeJSON,
	reportseps.FormatCSV:      ContentTypeCSV,
}
