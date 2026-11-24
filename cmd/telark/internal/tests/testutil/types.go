package testutil

import (
	"bytes"
	"net/http"
	"net/url"
	"sync"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
)

type Result struct {
	Stdout string
	Stderr string
	Code   int
}

type FailingWriter struct{}

// Denied, Forbidden and Missing name resources as resource.group ("pods", "nodes"), and "version" stands for the
// server version. Missing ones are left out of discovery, as on a cluster without their CRD. User answers "who am I".
type Cluster struct {
	Objects     []runtime.Object
	Version     string
	User        string
	Denied      []string
	Forbidden   []string
	Missing     []string
	NoWatchList bool
}

// FakeCluster keeps what the CLI writes, so a test reads back what an install or upgrade left behind.
type FakeCluster struct {
	URL       string
	cluster   Cluster
	mutex     sync.Mutex
	objects   []storedObject
	refused   map[string]int32
	writes    []Write
	watchers  []*watcher
	generated int
}

type watcher struct {
	path     apiPath
	selector fields.Selector
	events   chan map[string]any
}

// Stream is a stdout that a test reads while the command still runs.
type Stream struct {
	mutex  sync.Mutex
	buffer bytes.Buffer
}

// Request reads like "PATCH configmaps/telark-settings".
type Write struct {
	Request string
	Query   url.Values
	Object  map[string]any
}

type storedObject struct {
	group     string
	resource  string
	namespace string
	name      string
	labels    map[string]string
	content   map[string]any
}

type servedResource struct {
	group      string
	version    string
	resource   string
	kind       string
	namespaced bool
}

type apiPath struct {
	group     string
	version   string
	namespace string
	resource  string
	name      string
}

// GitHub answers the update check in-process. Tests install it as http.DefaultTransport, which client-go also uses
// for a plain-HTTP API server, so every request to another host goes to Next.
type GitHub struct {
	Next     http.RoundTripper
	Status   int
	Body     string
	Err      error
	Requests []*http.Request
}

// Registry answers ghcr.io in-process, on the same http.DefaultTransport chain as GitHub.
type Registry struct {
	Next       http.RoundTripper
	tagsStatus int
	blobs      map[string]registryBlob
	tags       map[string]string
}

type registryBlob struct {
	mediaType string
	content   []byte
}

type ociManifest struct {
	SchemaVersion int             `json:"schemaVersion"`
	MediaType     string          `json:"mediaType"`
	Config        ociDescriptor   `json:"config"`
	Layers        []ociDescriptor `json:"layers"`
}

type ociDescriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int    `json:"size"`
}

type ociTags struct {
	Tags []string `json:"tags"`
}
