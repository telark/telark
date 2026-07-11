package protection

import (
	"encoding/json"

	"github.com/telark/data/plans"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ExtractFromUnstructured(resource *unstructured.Unstructured) (*plans.ProtectionPlan, error) {
	spec, ok := resource.Object["spec"].(map[string]any)
	if !ok || spec == nil {
		return nil, nil
	}

	specBytes, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}

	var plan plans.ProtectionPlan
	if err := json.Unmarshal(specBytes, &plan); err != nil {
		return nil, err
	}
	return &plan, nil
}
