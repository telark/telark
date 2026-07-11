package group

import (
	"net/http"

	metadata "github.com/telark/data/metadata/resources"
	"github.com/telark/exporter/constants"
	resourcesshared "github.com/telark/exporter/utils/resources/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func FindGroupByID(groupID string) (*unstructured.Unstructured, error) {
	return resourcesshared.FindResourceByID(groupID, metadata.GroupAsResourceMetadata, constants.ErrGroupNotFound)
}

func FindGroupByIDOrRespond(w http.ResponseWriter, groupID string) (*unstructured.Unstructured, bool) {
	return resourcesshared.FindResourceByIDOrRespond(w, groupID, metadata.GroupAsResourceMetadata, constants.ErrGroupNotFound)
}
