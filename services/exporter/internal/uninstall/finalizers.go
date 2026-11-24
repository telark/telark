package uninstall

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/telark/telark/internal/data/logger"
	metadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/data/resources/finalizers"
	"github.com/telark/telark/internal/kcore/crds/api"
	"github.com/telark/telark/services/exporter/internal/constants"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var targets = []target{
	{finalizers.ResourceTypeUsers, metadata.UserMetadata, finalizers.UserCleanup},
	{finalizers.ResourceTypeGroups, metadata.GroupMetadata, finalizers.GroupCleanup},
	{finalizers.ResourceTypeRoles, metadata.AccessRoleMetadata, finalizers.RoleCleanup},
}

// Best effort: a failure must not block the uninstall, and auth's sweeper
// restores any finalizer removed here if the release is installed again.
func RemoveFinalizers(lg *logger.CustomLogger) Result {
	result := Result{}
	for _, t := range targets {
		listed := api.ListCustomResources(t.metadata)
		list, ok := listed.Data.(*unstructured.UnstructuredList)
		if listed.Status != http.StatusOK || !ok {
			lg.Error(fmt.Sprintf(string(constants.ErrRemoveFinalizersListFailed), t.resourceType, listed.Message))
			result.Failed++
			continue
		}
		for i := range list.Items {
			obj := &list.Items[i]
			if !slices.Contains(obj.GetFinalizers(), t.finalizer) {
				continue
			}
			if !removeFinalizer(t, obj, lg) {
				result.Failed++
				continue
			}
			result.Removed++
		}
	}
	return result
}

func removeFinalizer(t target, obj *unstructured.Unstructured, lg *logger.CustomLogger) bool {
	kept := slices.DeleteFunc(slices.Clone(obj.GetFinalizers()), func(name string) bool { return name == t.finalizer })
	patch := map[string]any{constants.MetadataField: map[string]any{constants.FieldFinalizers: kept}}
	patched := api.PatchCustomResource(t.metadata, obj.GetName(), patch)
	// An object deleted since the list holds no finalizer anymore.
	if patched.Status != http.StatusOK && !k8serrors.IsNotFound(patched.Error) {
		lg.Error(fmt.Sprintf(string(constants.ErrRemoveFinalizerFailed), t.resourceType, obj.GetName(), patched.Message))
		return false
	}
	return true
}
