package role

import (
	"net/http"

	metadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/services/exporter/internal/constants"
	resourcesshared "github.com/telark/telark/services/exporter/internal/utils/resources/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func FindRoleByIDOrRespond(w http.ResponseWriter, roleID string) (*unstructured.Unstructured, bool) {
	return resourcesshared.FindResourceByIDOrRespond(w, roleID, metadata.AccessRoleMetadata, constants.ErrRoleNotFound)
}
