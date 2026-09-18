package protection

import (
	"github.com/telark/data/plans"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ExtractFromUnstructured(resource *unstructured.Unstructured) (*plans.ProtectionPlan, error) {
	return sharedutils.SpecToStruct[plans.ProtectionPlan](resource)
}
