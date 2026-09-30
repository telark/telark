package protection

import (
	"net/http"

	plansmd "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/services/exporter/internal/constants"
	resourcesshared "github.com/telark/telark/services/exporter/internal/utils/resources/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func FindByIDOrRespond(w http.ResponseWriter, planID string) (*unstructured.Unstructured, bool) {
	return resourcesshared.FindResourceByIDOrRespond(w, planID, plansmd.ProtectionPlanMetadata, constants.ErrProtectionPlanNotFound)
}
