package core

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/data/resources/application"
	natscore "github.com/telark/x-ware/nats/core"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type GetApplicationsOptions struct {
	// Wait enables polling Redis until all apps are enriched or timeout.
	Wait bool
	// WaitTimeoutSec is the max number of seconds to wait (default 15, max 30).
	WaitTimeoutSec int
	// InsightsEnabled toggles enrichment (Redis cache + queue). When false, insights stay empty.
	InsightsEnabled bool
	// NatsClient, when set, is used to publish applications to NATS for CR creation.
	NatsClient *natscore.NATSClient
	// GetStoredApplication fetches the current Application CR state by name (e.g. from exporter API).
	// Use a client with 3s timeout. If nil or returns nil/error, history is set to new-app (generation 1, no snapshot).
	GetStoredApplication func(name string) *application.Application
	// CreateSnapshot stores manifests in exporter snapshot storage and returns the stored path.
	CreateSnapshot func(id string, scope string, namespace string, generation int, manifest any) (string, error)
	// DeleteSnapshot removes a snapshot file from exporter snapshot storage. Used to take back files
	// written ahead of a diff that then authored nothing. When nil, those files are left in place.
	DeleteSnapshot func(id string, scope string, namespace string, generation int) error
	// GetSnapshotManifest fetches a previously stored snapshot manifest from exporter snapshot storage (optional).
	// When set, it is used to ensure "pre-change" snapshots reflect the previous known state.
	GetSnapshotManifest func(
		ctx context.Context,
		snapshotID string,
		scope string,
		namespace string,
		generation int,
	) ([]unstructured.Unstructured, error)
	// RedisClient enables scaling grace and incident/recovery deduplication for change history (optional).
	RedisClient                  *redis.Client
	PrewrittenSnapshotAppName    string
	PrewrittenSnapshotGeneration int
	PrewrittenSnapshots          []application.ApplicationSnapshot
	// FromCoalescingFlush marks calls from the informer coalescing flush path. When true, snapshot
	// fallback writes and best-effort backfill are suppressed; the valid pre-change manifest source
	// is the prewritten snapshot from oldObj (when provided).
	FromCoalescingFlush bool
	// FromForceSync marks calls from the coordination force-sync handler (leader-only full reconcile).
	// Diff/snapshot behavior uses this to avoid contending on per-generation processing locks held
	// elsewhere on that path.
	FromForceSync bool
}

type resourceAggregate struct {
	nsCounts        map[string]int
	kindCounts      map[string]int
	resList         []application.Resource
	createdAt       time.Time
	lastUpdated     time.Time
	lastModifiedAt  time.Time
	lastModifiedBy  string
	lastModifiedKey string
	imagesSet       map[string]bool
	portsSet        map[int]bool
	envKeysSet      map[string]bool
	configMapRefs   map[string]bool
	secretRefs      map[string]bool
	serviceMappings map[string]bool
	ingressRules    map[string]bool
}

type labelValues struct {
	managedBy   string
	chartVal    string
	versionVal  string
	displayName string
}

// Labels & managed-by
const (
	labelManagedBy          = "app.kubernetes.io/managed-by"
	labelChart              = "helm.sh/chart"
	labelVersion            = "app.kubernetes.io/version"
	labelAppName            = "app.kubernetes.io/name"
	helmReleaseSecretPrefix = "sh.helm.release."
)

// Wait / polling
const (
	sortOne             = 1
	firstItemIdx        = 0
	waitPollIntervalSec = 1
)

// Enrich K8s
const (
	envKeySkipSubstr = "PASSWORD,SECRET,TOKEN,KEY,CREDENTIAL"
	keySeparator     = "\x00"
)

type enrichResult struct {
	createdAt       time.Time
	lastModifiedBy  string
	lastModifiedAt  time.Time
	lastModifiedOp  string
	spec            *corev1.PodSpec
	configMapRefs   []string
	secretRefs      []string
	serviceMappings []string
	ingressRules    []string
}

// API response
const (
	operationSuccess             = "Success"
	messageResourcesGroupedByApp = "resources grouped by application"
)

// Log / message formats
const (
	msgEnqueueFailed    = "enqueue failed: app=%s err=%v"
	msgReplicasReady    = "%d/%d replicas ready"
	payloadKeyName      = "name"
	payloadKeyHistory   = "history"
	payloadKeySnapshots = "snapshots"
	payloadKeyTakenAt   = "takenAt"
	csvSeparator        = ","
)
