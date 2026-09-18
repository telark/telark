package group

import (
	"net/http"

	metadata "github.com/telark/data/metadata/resources"
	"github.com/telark/exporter/internal/constants"
	resourcesshared "github.com/telark/exporter/internal/utils/resources/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func FindGroupByIDOrRespond(w http.ResponseWriter, groupID string) (*unstructured.Unstructured, bool) {
	return resourcesshared.FindResourceByIDOrRespond(w, groupID, metadata.GroupAsResourceMetadata, constants.ErrGroupNotFound)
}
