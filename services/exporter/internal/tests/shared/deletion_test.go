package shared

import (
	"testing"
	"time"

	"github.com/telark/telark/services/exporter/internal/constants"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const deletedAt = "2026-09-26T10:00:00Z"

func terminatingRecord(t *testing.T) *unstructured.Unstructured {
	item := &unstructured.Unstructured{Object: map[string]any{
		constants.SpecField: map[string]any{constants.FieldName: "gone"},
	}}
	item.SetName("gone")
	parsed, err := time.Parse(time.RFC3339, deletedAt)
	if err != nil {
		t.Fatalf("parse %s: %v", deletedAt, err)
	}
	deleted := metav1.NewTime(parsed)
	item.SetDeletionTimestamp(&deleted)
	return item
}

// Peers read the record over HTTP and the in-process reader decodes the spec;
// both must see the deletion, and neither may write it into a shared spec map.
func TestDeletionTimestampProjection(t *testing.T) {
	item := terminatingRecord(t)
	single, err := sharedutils.FilterData(item)
	if err != nil {
		t.Fatalf("FilterData: %v", err)
	}
	if got := single.(*unstructured.Unstructured).Object[constants.FieldDeletionTimestamp]; got != deletedAt {
		t.Fatalf("single deletionTimestamp = %v", got)
	}

	list, err := sharedutils.FilterData(&unstructured.UnstructuredList{Items: []unstructured.Unstructured{*item}})
	if err != nil {
		t.Fatalf("FilterData list: %v", err)
	}
	for _, listed := range list.(*unstructured.UnstructuredList).Items {
		if got := listed.Object[constants.FieldDeletionTimestamp]; got != deletedAt {
			t.Fatalf("list deletionTimestamp = %v", got)
		}
	}
	if _, leaked := item.Object[constants.SpecField].(map[string]any)[constants.FieldDeletionTimestamp]; leaked {
		t.Fatal("FilterData wrote into the source spec")
	}

	sharedutils.ProjectDeletionTimestamp(item)
	if got := item.Object[constants.SpecField].(map[string]any)[constants.FieldDeletionTimestamp]; got != deletedAt {
		t.Fatalf("projected deletionTimestamp = %v", got)
	}
	live := &unstructured.Unstructured{Object: map[string]any{constants.SpecField: map[string]any{}}}
	sharedutils.ProjectDeletionTimestamp(live)
	if _, present := live.Object[constants.SpecField].(map[string]any)[constants.FieldDeletionTimestamp]; present {
		t.Fatal("live record gained a deletion timestamp")
	}
}
