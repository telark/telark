package snapshot

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/telark/exporter/internal/constants"
	envmanager "github.com/telark/exporter/internal/managers/envs"
	"github.com/telark/exporter/internal/utils/artifact"
)

func BuildSnapshotDir(
	snapshotsPath string,
	scope string,
	id string,
	namespace string,
	namespaced bool,
) string {
	if namespaced {
		return filepath.Join(snapshotsPath, scope, id, namespace)
	}
	return filepath.Join(snapshotsPath, scope, id)
}

func BuildSnapshotPath(
	snapshotsPath string,
	scope string,
	id string,
	namespace string,
	generation int,
	namespaced bool,
) string {
	return filepath.Join(
		BuildSnapshotDir(snapshotsPath, scope, id, namespace, namespaced),
		fmt.Sprintf(constants.SnapshotFileNameTemplate, generation),
	)
}

func ValidateSnapshotIdentity(id string, scope string) error {
	if id == constants.EmptyString {
		return errors.New(string(constants.ErrSnapshotIDSimpleRequired))
	}
	if scope == constants.EmptyString {
		return errors.New(string(constants.ErrSnapshotScopeSimpleRequired))
	}
	if !artifact.IsSafeSegment(id) {
		return errors.New(string(constants.ErrSnapshotIDSimpleRequired))
	}
	return nil
}

func APISnapshotPath(scope string, id string, namespace string, generation int, namespaced bool) string {
	return BuildSnapshotPath(envmanager.GetSnapshotsPath(), scope, id, namespace, generation, namespaced)
}
