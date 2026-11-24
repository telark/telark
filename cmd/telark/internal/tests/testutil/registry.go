package testutil

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"maps"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"helm.sh/helm/v4/pkg/chart/v2/loader"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/registry"
)

const (
	registryHost   = "ghcr.io"
	manifestType   = "application/vnd.oci.image.manifest.v1+json"
	digestHeader   = "Docker-Content-Digest"
	lengthHeader   = "Content-Length"
	digestPrefix   = "sha256:"
	tagsResource   = "tags"
	schemaVersion  = 2
	chartDirPrefix = "telark-test-chart"
	notAChart      = "not a chart archive"
)

// It swaps a process-wide transport, so only a TestMain may call it.
func InstallRegistry() *Registry {
	fake := &Registry{Next: http.DefaultTransport, tagsStatus: http.StatusOK, blobs: map[string]registryBlob{}, tags: map[string]string{}}
	http.DefaultTransport = fake
	return fake
}

func (r *Registry) Publish(directory string, versions ...string) {
	scratch := must(os.MkdirTemp("", chartDirPrefix))
	defer func() { _ = os.RemoveAll(scratch) }()
	for _, version := range versions {
		chart := must(loader.LoadDir(directory))
		chart.Metadata.Version = version
		archive := must(os.ReadFile(must(chartutil.Save(chart, scratch))))
		r.push(version, must(json.Marshal(chart.Metadata)), archive)
	}
}

func (r *Registry) PublishBroken(tag string) {
	r.push(tag, []byte("{}"), []byte(notAChart))
}

func (r *Registry) push(tag string, config, archive []byte) {
	manifest := must(json.Marshal(ociManifest{
		SchemaVersion: schemaVersion, MediaType: manifestType,
		Config: r.store(registry.ConfigMediaType, config), Layers: []ociDescriptor{r.store(registry.ChartLayerMediaType, archive)},
	}))
	r.tags[tag] = r.store(manifestType, manifest).Digest
}

func (r *Registry) store(mediaType string, content []byte) ociDescriptor {
	sum := sha256.Sum256(content)
	digest := digestPrefix + hex.EncodeToString(sum[:])
	r.blobs[digest] = registryBlob{mediaType: mediaType, content: content}
	return ociDescriptor{MediaType: mediaType, Digest: digest, Size: len(content)}
}

func (r *Registry) Digest(tag string) string {
	return r.tags[tag]
}

func (r *Registry) FailTags(t testing.TB) {
	r.tagsStatus = http.StatusForbidden
	t.Cleanup(func() { r.tagsStatus = http.StatusOK })
}

func (r *Registry) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Host != registryHost {
		return r.Next.RoundTrip(request)
	}
	parts := strings.Split(request.URL.Path, pathSeparator)
	kind, reference := parts[len(parts)-coreSkip], parts[len(parts)-second]
	if kind == tagsResource {
		tags := must(json.Marshal(ociTags{Tags: slices.Sorted(maps.Keys(r.tags))}))
		return respond(request, r.tagsStatus, jsonType, string(tags)), nil
	}
	digest := cmp.Or(r.tags[reference], reference)
	blob, found := r.blobs[digest]
	if !found {
		return respond(request, http.StatusNotFound, jsonType, ""), nil
	}
	response := respond(request, http.StatusOK, blob.mediaType, string(blob.content))
	response.ContentLength = int64(len(blob.content))
	response.Header.Set(digestHeader, digest)
	response.Header.Set(lengthHeader, strconv.Itoa(len(blob.content)))
	return response, nil
}

func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}
