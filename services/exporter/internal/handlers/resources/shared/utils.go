package shared

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/telark/data/errors"
	metadata "github.com/telark/data/metadata/base"
	crdapi "github.com/telark/kcore/crds/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ConvertToResource[T any](u *unstructured.Unstructured) (*T, error) {
	jsonBytes, err := u.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf(string(errors.ErrRestMarshalUnstructuredToJSON), err)
	}

	var resource T
	if err := json.Unmarshal(jsonBytes, &resource); err != nil {
		return nil, fmt.Errorf(string(errors.ErrRestUnmarshalResourceToJSON), err)
	}

	return &resource, nil
}

func CreateGenericPatchFunction(resourceMetadata metadata.Metadata) func(string, map[string]any) error {
	return func(name string, patch map[string]any) error {
		result := crdapi.PatchCustomResource(resourceMetadata, name, patch)
		if result.Status != http.StatusOK {
			return fmt.Errorf(string(errors.ErrPatchRes), name, strconv.Itoa(result.Status))
		}
		return nil
	}
}
