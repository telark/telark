package manifestdiff

import (
	"github.com/telark/discovery/internal/constants"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Either side nil skips the pair: adds and deletes are already reported as topology changes.
type ManifestPair struct {
	Old *unstructured.Unstructured
	New *unstructured.Unstructured
}

type diffRow struct {
	path       string
	changeType string
	oldValue   *string
	newValue   *string
}

const (
	rootSpec       = "spec"
	rootData       = "data"
	rootMetadata   = "metadata"
	labelsKey      = "labels"
	annotationsKey = "annotations"
	itemNameKey    = "name"
	pathSeparator  = "."
	pathLabels     = rootMetadata + pathSeparator + labelsKey
	pathAnnotation = rootMetadata + pathSeparator + annotationsKey
	indexOpen      = "["
	indexClose     = "]"
	kindNameSep    = "/"
	descTransition = " → "
	descAdded      = " added: "
	descRemoved    = " removed: "
	descColon      = ": "
	maxValueRunes  = 200
	ellipsis       = "…"

	kindService = "Service"
	kindIngress = "Ingress"
	kindSecret  = "Secret"

	redactedValue  = "<redacted>"
	rootStringData = "stringData"
)

// Controller-managed annotations that churn without a user change.
var noisyAnnotations = []string{
	"deployment.kubernetes.io/revision",
	"kubectl.kubernetes.io/last-applied-configuration",
	constants.AnnotationLastModifiedAt,
	constants.AnnotationLastModifiedBy,
	constants.AnnotationLastModifiedOperation,
}

// API-server defaults the exporter strips from served snapshot manifests (utils/snapshot/sanitize.go),
// so a stored copy only compares equal to a live Service without them.
var servedServiceSpecStripped = []string{"internalTrafficPolicy", "ipFamilies", "ipFamilyPolicy", "sessionAffinity"}

// Paths the curated summary checks already report; skipped here so a change is never listed twice.
var curatedPathSuffixes = []string{
	"].image",
	"].ports",
	"].env",
	"].envFrom",
	"].resources",
	"].configMap",
	"].secret",
}

var curatedExactPaths = []string{
	"spec.replicas",
}

var curatedKindPaths = map[string][]string{
	kindService: {"spec.ports", "spec.selector"},
	kindIngress: {"spec.rules"},
}
