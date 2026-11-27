package testutil

import (
	"encoding/json"
	"maps"
	"net/http"
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	keyObject          = "object"
	initialVersion     = "1"
	fieldParam         = "fieldSelector"
	initialEventsParam = "sendInitialEvents"
	nameField          = "metadata.name"
	namespaceField     = "metadata.namespace"
	watchListRefused   = "sendInitialEvents is forbidden for watch unless the WatchList feature gate is enabled"
	watchBuffer        = 64
	oneObject          = 1
)

// client-go's informers open a watch that first lists: the stored objects as ADDED events, then the bookmark that
// ends them. Each later change to a matching object follows as its own event, until the client leaves.
func (f *FakeCluster) watch(w http.ResponseWriter, r *http.Request, path apiPath) {
	if f.cluster.NoWatchList && r.URL.Query().Get(initialEventsParam) == annotationTrue {
		// An API server without watch lists refuses the parameter, and client-go then lists before it watches.
		writeJSON(w, http.StatusUnprocessableEntity, failure(http.StatusUnprocessableEntity, metav1.StatusReasonInvalid, watchListRefused))
		return
	}
	current, subscriber := f.subscribe(path, fields.ParseSelectorOrDie(r.URL.Query().Get(fieldParam)))
	defer f.unsubscribe(subscriber)
	w.Header().Set(contentType, jsonType)
	w.WriteHeader(http.StatusOK)
	events := json.NewEncoder(w)
	for _, content := range current {
		_ = events.Encode(map[string]any{keyType: watch.Added, keyObject: content})
	}
	bookmark := unstructured.Unstructured{}
	bookmark.SetAPIVersion(path.groupVersion())
	bookmark.SetKind(servedKind(path))
	bookmark.SetResourceVersion(initialVersion)
	bookmark.SetAnnotations(map[string]string{metav1.InitialEventsAnnotationKey: annotationTrue})
	_ = events.Encode(map[string]any{keyType: watch.Bookmark, keyObject: bookmark.Object})
	flusher := http.NewResponseController(w)
	_ = flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-subscriber.events:
			_ = events.Encode(event)
			_ = flusher.Flush()
		}
	}
}

func (f *FakeCluster) subscribe(path apiPath, selector fields.Selector) ([]map[string]any, *watcher) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	subscriber := &watcher{path: path, selector: selector, events: make(chan map[string]any, watchBuffer)}
	f.watchers = append(f.watchers, subscriber)
	var contents []map[string]any
	for _, object := range f.objects {
		if subscriber.wants(object) {
			contents = append(contents, object.content)
		}
	}
	return contents, subscriber
}

func (f *FakeCluster) unsubscribe(subscriber *watcher) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.watchers = slices.DeleteFunc(f.watchers, func(other *watcher) bool { return other == subscriber })
}

// Called with the mutex held. A full buffer drops the event rather than block every other request.
func (f *FakeCluster) notify(kind watch.EventType, object storedObject) {
	for _, subscriber := range f.watchers {
		if !subscriber.wants(object) {
			continue
		}
		select {
		case subscriber.events <- map[string]any{keyType: kind, keyObject: object.content}:
		default:
		}
	}
}

func (s *watcher) wants(object storedObject) bool {
	return object.matches(s.path) && s.selector.Matches(object.fields())
}

func (o storedObject) fields() fields.Set {
	return fields.Set{nameField: o.name, namespaceField: o.namespace}
}

// SetStatus writes a status the way an object's controller does, and every watch on the object sees the change. The
// stored content is replaced, never changed in place, since a watch may still be encoding the old one.
func (f *FakeCluster) SetStatus(t *testing.T, resource, name string, status map[string]any) {
	t.Helper()
	f.mutex.Lock()
	defer f.mutex.Unlock()
	index := f.indexOf(t, resource, name)
	content := maps.Clone(f.objects[index].content)
	content[keyStatus] = status
	f.objects[index].content = content
	f.notify(watch.Modified, f.objects[index])
}

func (f *FakeCluster) Remove(t *testing.T, resource, name string) {
	t.Helper()
	f.mutex.Lock()
	defer f.mutex.Unlock()
	index := f.indexOf(t, resource, name)
	removed := f.objects[index]
	f.objects = slices.Delete(f.objects, index, index+oneObject)
	f.notify(watch.Deleted, removed)
}

func (f *FakeCluster) indexOf(t *testing.T, resource, name string) int {
	t.Helper()
	index := slices.IndexFunc(f.objects, func(object storedObject) bool { return object.resource == resource && object.name == name })
	if index == missingObject {
		t.Fatalf(notFoundFormat, resource, name)
	}
	return index
}
