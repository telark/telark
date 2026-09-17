package manifestdiff

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

// ManifestPair is one object before (informer-captured oldObject) and after
// (informer cache) a change. Either side nil means the pair is skipped: adds
// and deletes are already reported as topology changes.
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
}

// Paths already reported by the curated summary checks (replicas, images,
// ports, env keys, config/secret refs, resources, service mappings, ingress
// rules). The generic layer skips them so a change is never listed twice.
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
