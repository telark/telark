package protection

import (
	"net/http"

	plansmd "github.com/telark/data/metadata/plans"
	"github.com/telark/exporter/constants"
	resourcesshared "github.com/telark/exporter/utils/resources/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func FindByID(planID string) (*unstructured.Unstructured, error) {
	return resourcesshared.FindResourceByID(planID, plansmd.ProtectionPlanMetadata, constants.ErrProtectionPlanNotFound)
}

func FindByIDOrRespond(w http.ResponseWriter, planID string) (*unstructured.Unstructured, bool) {
	return resourcesshared.FindResourceByIDOrRespond(w, planID, plansmd.ProtectionPlanMetadata, constants.ErrProtectionPlanNotFound)
}
