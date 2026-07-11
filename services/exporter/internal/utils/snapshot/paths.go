package snapshot

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/telark/exporter/constants"
	envmanager "github.com/telark/exporter/managers/envs"
)

func IsWithinBase(targetPath string, basePath string) bool {
	return len(targetPath) > len(basePath) &&
		targetPath[:len(basePath)+baseSeparatorShift] == basePath+string(os.PathSeparator)
}

//nolint:revive
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

//nolint:revive
func BuildSnapshotPath(
	snapshotsPath string,
	scope string,
	id string,
	namespace string,
	generation int,
	namespaced bool,
) string {
	if namespaced {
		return filepath.Join(
			snapshotsPath,
			scope,
			id,
			namespace,
			fmt.Sprintf("V%d.json", generation),
		)
	}
	return filepath.Join(
		snapshotsPath,
		scope,
		id,
		fmt.Sprintf("V%d.json", generation),
	)
}

func ValidateSnapshotIdentity(id string, scope string) error {
	if id == constants.EmptyString {
		return errors.New(string(constants.ErrSnapshotIDSimpleRequired))
	}
	if scope == constants.EmptyString {
		return errors.New(string(constants.ErrSnapshotScopeSimpleRequired))
	}
	cleanID := filepath.Base(id)
	if cleanID == "." || cleanID == ".." || cleanID != id {
		return errors.New(string(constants.ErrSnapshotIDSimpleRequired))
	}
	return nil
}

func APISnapshotPath(scope string, id string, namespace string, generation int, namespaced bool) string {
	return BuildSnapshotPath(envmanager.GetSnapshotsPath(), scope, id, namespace, generation, namespaced)
}
