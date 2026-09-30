package core

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/internal/data/resources/application"
	natscore "github.com/telark/telark/internal/x-ware/nats/core"
	"github.com/telark/telark/services/discovery/internal/core/applications/history/diff"
	"github.com/telark/telark/services/discovery/internal/core/applications/history/manifestdiff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type GetApplicationsOptions struct {
	NatsClient *natscore.NATSClient
	// (nil, nil) is a new application (generation 1); an error skips the application entirely so
	// a transient exporter outage can never reset stored history.
	GetStoredApplication func(name string) (*application.Application, error)
	CreateSnapshot       func(id string, scope string, namespace string, generation int, manifest any) (string, error)
	// Takes back files written ahead of a diff that then authored nothing; nil leaves them in place.
	DeleteSnapshot func(id string, scope string, namespace string, generation int) error
	// When set, "pre-change" snapshots reflect the previous known state instead of the live one.
	GetSnapshotManifest func(
		ctx context.Context,
		snapshotID string,
		scope string,
		namespace string,
		generation int,
	) ([]unstructured.Unstructured, error)
	// Optional; enables scaling grace and incident/recovery deduplication for change history.
	RedisClient                  *redis.Client
	PrewrittenSnapshotAppName    string
	PrewrittenSnapshotGeneration int
	PrewrittenSnapshots          []application.ApplicationSnapshot
	// Informer-captured before/after objects behind a coalescing flush; the generic manifest
	// diff reports every field change between them.
	ManifestPairs []manifestdiff.ManifestPair
	// Suppresses snapshot fallback writes and best-effort backfill: the only valid pre-change
	// manifest source on this path is the prewritten snapshot from oldObj.
	FromCoalescingFlush bool
	// Keeps the diff off the per-generation processing locks the force-sync path already holds.
	FromForceSync bool
	// Set while a rollback applies: the flush records its writes as that rollback's entry.
	Rollback *diff.RollbackMarker
	// Grouped applications with health and nothing else (no diff, snapshots, metrics or publish):
	// the leader tick only enumerates apps, per-app consumer jobs do the expensive work.
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

type metaObject interface {
	GetName() string
	GetAnnotations() map[string]string
	GetCreationTimestamp() metav1.Time
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
