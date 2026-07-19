package startup

import (
	"fmt"
	"net/http"

	categorydata "github.com/telark/data/classification/category"
	classificationmeta "github.com/telark/data/metadata/classification"
	resourcesmeta "github.com/telark/data/metadata/resources"

	basemeta "github.com/telark/data/metadata/base"
	globalconfigresource "github.com/telark/data/resources/globalconfig"
	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/internal/constants"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/kcore/crds/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var lg = constants.GetLogger(constants.PrefixStartup)

// The built-in definitions are the permission model, so they are reconciled on every
// boot rather than created once: an edit made straight against the API is undone at
// the next restart.
func SeedBuiltins() {
	lg.Info(string(constants.InfSeedStarting))
	seedRoles()
	seedCategories()
	seedGlobalConfig()
}

func seedRoles() {
	for _, builtin := range roledata.BuiltinRoles {
		spec, err := sharedutils.StructToSpecMap(builtin)
		if err != nil {
			lg.Error(fmt.Sprintf(string(constants.ErrSeedSpecEncodeFailed), builtin.ID, err))
			continue
		}

		if err := upsert(resourcesmeta.RoleAsResourceMetadata, builtin.ID, spec); err != nil {
			lg.Error(fmt.Sprintf(string(constants.ErrSeedRoleFailed), builtin.ID, err))
			continue
		}
		lg.Info(fmt.Sprintf(string(constants.InfSeedRoleReconciled), builtin.Name))
	}
}

// Built-ins and user-created categories share one resource, so the built-ins are
// restored in place and everything else is carried across untouched.
func seedCategories() {
	md := classificationmeta.CategoryAsClassificationMetadata

	existing, err := currentCategories(md)
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrSeedCategoriesFailed), err))
		return
	}

	spec, err := sharedutils.StructToSpecMap(categorydata.CategoryAsClassification{
		Categories: categorydata.WithBuiltins(existing),
	})
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrSeedCategoriesFailed), err))
		return
	}

	if err := upsert(md, constants.CategoriesCRDName, spec); err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrSeedCategoriesFailed), err))
		return
	}
	lg.Info(fmt.Sprintf(string(constants.InfSeedCategoriesMerged), len(categorydata.BuiltinCategories)))
}

func currentCategories(md basemeta.Metadata) ([]categorydata.Category, error) {
	exists, err := api.CheckCustomResourceExistsByName(constants.CategoriesCRDName, md)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}

	result := api.GetCustomResourceByName(constants.CategoriesCRDName, md)
	if result.Error != nil {
		return nil, result.Error
	}
	resource, ok := result.Data.(*unstructured.Unstructured)
	if !ok || resource == nil {
		return nil, nil
	}

	current, err := sharedutils.UnstructuredToStruct[categorydata.CategoryAsClassification](
		resource,
		constants.ErrCategoriesSpecNotFound,
		constants.ErrCategoriesSpecInvalid,
		constants.ErrFailedToUnmarshalCategory,
	)
	if err != nil {
		return nil, err
	}
	return current.Categories, nil
}

// Created once and never reconciled: cluster version, AI settings, display
// preferences and the identity provider are all written after creation, and
// rewriting the defaults would discard them.
func seedGlobalConfig() {
	md := resourcesmeta.GlobalConfigMetadata

	exists, err := api.CheckCustomResourceExistsByName(constants.GlobalConfigResourceName, md)
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrSeedExistsCheckFailed), constants.GlobalConfigResourceName, err))
		return
	}
	if exists {
		lg.Info(string(constants.InfSeedGlobalConfigKept))
		return
	}

	spec, err := sharedutils.StructToSpecMap(globalconfigresource.DefaultGlobalConfig())
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrSeedGlobalConfigFail), err))
		return
	}

	if err := create(md, constants.GlobalConfigResourceName, spec); err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrSeedGlobalConfigFail), err))
		return
	}
	lg.Info(string(constants.InfSeedGlobalConfigOK))
}

func upsert(md basemeta.Metadata, name string, spec map[string]any) error {
	exists, err := api.CheckCustomResourceExistsByName(name, md)
	if err != nil {
		return fmt.Errorf(string(constants.ErrSeedExistsCheckFailed), name, err)
	}
	if !exists {
		return create(md, name, spec)
	}

	result := api.PatchCustomResource(md, name, map[string]any{constants.SpecField: spec})
	return resultError(result.Status, result.Error)
}

func create(md basemeta.Metadata, name string, spec map[string]any) error {
	result := api.CreateCustomResource(sharedutils.ConvertToCRDTemplate(md, name, spec), md)
	// A concurrent replica winning the create is the expected outcome, not a failure.
	if result.Status == http.StatusConflict {
		return nil
	}
	return resultError(result.Status, result.Error)
}

func resultError(status int, err error) error {
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("%s", http.StatusText(status))
	}
	return nil
}
