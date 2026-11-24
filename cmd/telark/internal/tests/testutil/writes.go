package testutil

import (
	"fmt"
	"io"
	"net/http"
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/yaml"
)

const (
	protobufType    = "application/vnd.kubernetes.protobuf"
	dryRunParam     = "dryRun"
	missingObject   = -1
	generatedFormat = "%s%05d"
)

func (f *FakeCluster) write(w http.ResponseWriter, r *http.Request, path apiPath) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	code, body := f.change(r, path)
	writeJSON(w, code, body)
}

func (f *FakeCluster) change(r *http.Request, path apiPath) (code int, body any) {
	if r.Method == http.MethodDelete {
		f.objects = slices.DeleteFunc(f.objects, func(object storedObject) bool {
			deleted := object.matches(path) && object.name == path.name
			if deleted {
				f.notify(watch.Deleted, object)
			}
			return deleted
		})
		return http.StatusOK, metav1.Status{TypeMeta: metav1.TypeMeta{APIVersion: coreVersion, Kind: "Status"}, Status: metav1.StatusSuccess}
	}
	object, err := decode(r, path)
	if err != nil {
		return http.StatusBadRequest, failure(http.StatusBadRequest, metav1.StatusReasonBadRequest, err.Error())
	}
	f.generateName(&object)
	existing := f.index(apiPath{group: path.group, resource: path.resource, namespace: path.namespace, name: object.name})
	f.writes = append(f.writes, Write{Request: path.request(r.Method), Query: r.URL.Query(), Object: object.content})
	stored := f.keep(object, existing, r.URL.Query().Has(dryRunParam))
	if existing == missingObject {
		return http.StatusCreated, stored
	}
	return http.StatusOK, stored
}

// An apply or update keeps the status it doesn't send, and the API server establishes a CRD it accepts.
func (f *FakeCluster) keep(object storedObject, existing int, dryRun bool) map[string]any {
	if _, sent := object.content[keyStatus]; !sent && existing != missingObject {
		object.content[keyStatus] = f.objects[existing].content[keyStatus]
	}
	if _, set := object.content[keyStatus]; !set && object.resource == crdResource {
		object.content[keyStatus] = establishedStatus(conditionTrue)
	}
	switch {
	case dryRun:
	case existing == missingObject:
		f.objects = append(f.objects, object)
		f.notify(watch.Added, object)
	default:
		f.objects[existing] = object
		f.notify(watch.Modified, object)
	}
	return object.content
}

// The API server appends five random characters to generateName; a counter keeps the names predictable.
func (f *FakeCluster) generateName(object *storedObject) {
	item := unstructured.Unstructured{Object: object.content}
	if object.name != "" || item.GetGenerateName() == "" {
		return
	}
	f.generated++
	object.name = fmt.Sprintf(generatedFormat, item.GetGenerateName(), f.generated)
	item.SetName(object.name)
}

func (f *FakeCluster) index(path apiPath) int {
	return slices.IndexFunc(f.objects, func(object storedObject) bool { return object.matches(path) && object.name == path.name })
}

// Typed clients send built-in kinds as protobuf. Helm sends its objects, CRDs included, as JSON, which YAML reads too.
func decode(r *http.Request, path apiPath) (storedObject, error) {
	item := unstructured.Unstructured{}
	body, err := io.ReadAll(r.Body)
	if err == nil && r.Header.Get(contentType) == protobufType {
		item.Object, err = typed(body)
	} else if err == nil {
		err = yaml.Unmarshal(body, &item.Object)
	}
	return storedObject{
		group: path.group, resource: path.resource, namespace: path.namespace, name: item.GetName(),
		labels: item.GetLabels(), content: item.Object,
	}, err
}

func typed(body []byte) (map[string]any, error) {
	object, gvk, err := scheme.Codecs.UniversalDeserializer().Decode(body, nil, nil)
	if err != nil {
		return nil, err
	}
	object.GetObjectKind().SetGroupVersionKind(*gvk)
	return runtime.DefaultUnstructuredConverter.ToUnstructured(object)
}

func (f *FakeCluster) Writes() []Write {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return slices.Clone(f.writes)
}

func (f *FakeCluster) Find(resource, name string) map[string]any {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	for _, object := range f.objects {
		if object.resource == resource && object.name == name {
			return object.content
		}
	}
	return nil
}
