package testutil

import (
	"net/http"
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/telark/telark/internal/data/metadata/base"
)

const (
	coreDiscovery   = "/api"
	groupDiscovery  = "/apis"
	crdGroup        = "apiextensions.k8s.io"
	crdResource     = "customresourcedefinitions"
	coreVersion     = "v1"
	openAPIResource = "openapi"
	openAPIType     = "application/com.github.proto-openapi.spec.v2.v1.0+protobuf"
)

// Helm maps every kind of a chart through discovery, so the fake serves the kinds the test charts hold.
var served = []servedResource{
	{version: coreVersion, resource: "namespaces", kind: "Namespace"},
	{version: coreVersion, resource: "configmaps", kind: "ConfigMap", namespaced: true},
	{version: coreVersion, resource: "secrets", kind: "Secret", namespaced: true},
	{group: crdGroup, version: coreVersion, resource: crdResource, kind: crdKind},
	{group: base.Group, version: base.V1Alpha1, resource: "categories", kind: "Category", namespaced: true},
	{group: base.Group, version: base.V1Alpha1, resource: "authrequests", kind: "AuthRequest", namespaced: true},
	{group: base.Group, version: base.V1Alpha1, resource: "telarkconfigs", kind: "TelarkConfig", namespaced: true},
}

func discovers(urlPath string, path apiPath) bool {
	return urlPath == coreDiscovery || urlPath == groupDiscovery || path.resource == "" || path.resource == openAPIResource
}

func (f *FakeCluster) discover(w http.ResponseWriter, urlPath string, path apiPath) {
	switch {
	case path.resource == openAPIResource:
		// An empty OpenAPI document: Helm then checks no field on its side, and the fake takes any object.
		w.Header().Set(contentType, openAPIType)
	case urlPath == coreDiscovery:
		writeJSON(w, http.StatusOK, metav1.APIVersions{TypeMeta: metav1.TypeMeta{Kind: "APIVersions"}, Versions: []string{coreVersion}})
	case urlPath == groupDiscovery:
		writeJSON(w, http.StatusOK, groups())
	default:
		f.resources(w, path)
	}
}

func groups() metav1.APIGroupList {
	list := metav1.APIGroupList{TypeMeta: metav1.TypeMeta{Kind: "APIGroupList", APIVersion: coreVersion}}
	for _, resource := range served {
		named := func(group metav1.APIGroup) bool { return group.Name == resource.group }
		if resource.group == "" || slices.ContainsFunc(list.Groups, named) {
			continue
		}
		path := apiPath{group: resource.group, version: resource.version}
		version := metav1.GroupVersionForDiscovery{GroupVersion: path.groupVersion(), Version: resource.version}
		list.Groups = append(list.Groups, metav1.APIGroup{
			Name: resource.group, Versions: []metav1.GroupVersionForDiscovery{version}, PreferredVersion: version,
		})
	}
	return list
}

func (f *FakeCluster) resources(w http.ResponseWriter, path apiPath) {
	list := metav1.APIResourceList{TypeMeta: metav1.TypeMeta{Kind: "APIResourceList", APIVersion: coreVersion}}
	for _, resource := range served {
		missing := slices.Contains(f.cluster.Missing, apiPath{group: resource.group, resource: resource.resource}.qualified())
		if resource.group == path.group && resource.version == path.version && !missing {
			list.GroupVersion = path.groupVersion()
			list.APIResources = append(list.APIResources, metav1.APIResource{
				Name: resource.resource, Kind: resource.kind, Namespaced: resource.namespaced, Verbs: metav1.Verbs{"*"},
			})
		}
	}
	writeJSON(w, http.StatusOK, list)
}

func servedKind(path apiPath) string {
	return served[slices.IndexFunc(served, path.serves)].kind
}

func (p apiPath) serves(resource servedResource) bool {
	return resource.group == p.group && resource.resource == p.resource
}

func (p apiPath) groupVersion() string {
	return strings.TrimPrefix(p.group+pathSeparator+p.version, pathSeparator)
}
