package migrations

import (
	"errors"
	"fmt"
	"slices"

	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/kcore/crds/api"
	"github.com/telark/telark/services/exporter/internal/constants"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var lg = constants.GetLogger(constants.PrefixStartup)

// Append-only: TelarkConfig's status records a step by its Number, so a released step
// keeps its number and is never removed or reordered.
var Steps = []Step{}

// shortcut: no lock, so every replica that starts runs the pending steps and a crash before a
// record runs that step again; steps stay idempotent, or this needs a lease.
func Run(steps []Step) error {
	applied, rollback, err := lastApplied()
	if err != nil {
		return err
	}
	latest, previous := lastSteps(steps)
	if applied > latest {
		if rollback > latest {
			return fmt.Errorf(string(constants.ErrMigrationDataNewer), applied, latest, rollback)
		}
		// One release back: steps expand before they contract, so this code still reads the data.
		if err := record(latest, rollback); err != nil {
			return err
		}
		lg.Warn(fmt.Sprintf(string(constants.WarnMigrationRecordLowered), applied, latest))
	}
	for _, step := range steps {
		if step.Number <= applied {
			continue
		}
		if err := step.Apply(); err != nil {
			return fmt.Errorf(string(constants.ErrMigrationStepFailed), step.Number, step.Name, err)
		}
		if err := record(step.Number, previous); err != nil {
			return err
		}
		lg.Info(fmt.Sprintf(string(constants.InfMigrationStepApplied), step.Number, step.Name))
	}
	lg.Info(fmt.Sprintf(string(constants.InfMigrationsCurrent), latest))
	return nil
}

func lastSteps(steps []Step) (latest, previousRelease int64) {
	if len(steps) == constants.DefaultInitValue {
		return constants.DefaultInitValue, constants.DefaultInitValue
	}
	newest := steps[len(steps)-constants.DefaultIncrementValue]
	// A newest step without a Release allows no rollback over it, so a forgotten tag fails strict.
	if newest.Release == constants.EmptyString {
		return newest.Number, newest.Number
	}
	for _, step := range slices.Backward(steps) {
		if step.Release != newest.Release {
			return newest.Number, step.Number
		}
	}
	return newest.Number, constants.DefaultInitValue
}

func lastApplied() (step, rollback int64, err error) {
	result := api.GetCustomResourceByName(constants.TelarkConfigResourceName, v1alpha1.TelarkConfigMetadata)
	if result.Error != nil {
		return constants.DefaultInitValue, constants.DefaultInitValue, fmt.Errorf(string(constants.ErrMigrationRecordRead), result.Error)
	}
	if step, rollback, err = recorded(result.Data); err != nil {
		return constants.DefaultInitValue, constants.DefaultInitValue, fmt.Errorf(string(constants.ErrMigrationRecordRead), err)
	}
	return step, rollback, nil
}

func record(step, rollback int64) error {
	result := api.PatchCustomResourceStatus(v1alpha1.TelarkConfigMetadata, constants.TelarkConfigResourceName,
		map[string]any{constants.FieldStatus: map[string]any{
			constants.FieldLastMigration: step, constants.FieldRollbackMigration: rollback,
		}})
	if result.Error != nil {
		return fmt.Errorf(string(constants.ErrMigrationRecordWrite), step, result.Error)
	}
	// The API server prunes a field its schema lacks and still answers 200.
	if keptStep, keptRollback, err := recorded(result.Data); err != nil || keptStep != step || keptRollback != rollback {
		return errors.New(string(constants.ErrMigrationRecordDropped))
	}
	return nil
}

func recorded(data any) (step, rollback int64, err error) {
	config, ok := data.(*unstructured.Unstructured)
	if !ok || config == nil {
		return constants.DefaultInitValue, constants.DefaultInitValue, nil
	}
	if step, _, err = unstructured.NestedInt64(config.Object, constants.FieldStatus, constants.FieldLastMigration); err != nil {
		return constants.DefaultInitValue, constants.DefaultInitValue, err
	}
	rollback, _, err = unstructured.NestedInt64(config.Object, constants.FieldStatus, constants.FieldRollbackMigration)
	return step, rollback, err
}
