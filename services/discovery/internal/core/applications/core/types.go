package core

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/core/applications/history/manifestdiff"
	natscore "github.com/telark/x-ware/nats/core"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type GetApplicationsOptions struct {
	NatsClient *natscore.NATSClient
	// GetStoredApplication fetches the current Application CR state by name (e.g. from exporter API).
	// (nil, nil) means the application does not exist yet and gets a new-app history (generation 1).
	// An error means the state is unknown: the application is skipped entirely so a transient
	// exporter outage can never reset stored history.
	GetStoredApplication func(name string) (*application.Application, error)
	CreateSnapshot       func(id string, scope string, namespace string, generation int, manifest any) (string, error)
	// DeleteSnapshot removes a snapshot file from exporter snapshot storage. Used to take back files
	// written ahead of a diff that then authored nothing. When nil, those files are left in place.
	DeleteSnapshot func(id string, scope string, namespace string, generation int) error
	// When set, "pre-change" snapshots reflect the previous known state instead of the live one.
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
	// ManifestPairs are the informer-captured before/after objects behind a coalescing
	// flush; the generic manifest diff reports every field change between them.
	ManifestPairs []manifestdiff.ManifestPair
	// FromCoalescingFlush marks calls from the informer coalescing flush path. When true, snapshot
	// fallback writes and best-effort backfill are suppressed; the valid pre-change manifest source
	// is the prewritten snapshot from oldObj (when provided).
	FromCoalescingFlush bool
	// FromForceSync marks calls from the coordination force-sync handler (leader-only full reconcile).
	// Diff/snapshot behavior uses this to avoid contending on per-generation processing locks held
	// elsewhere on that path.
	FromForceSync bool
	// DeriveOnly returns grouped applications with health and nothing else: no
	// history diff, no snapshots, no metrics, no publish. The leader tick uses it
	// to enumerate apps; per-app consumer jobs do the expensive work.
	DeriveOnly bool
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
	msgReplicasReady    = "%d/%d replicas ready"
	payloadKeyName      = "name"
	payloadKeyHistory   = "history"
	payloadKeySnapshots = "snapshots"
	payloadKeyTakenAt   = "takenAt"
	csvSeparator        = ","
)
