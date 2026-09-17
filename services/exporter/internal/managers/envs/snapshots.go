package envs

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/telark/exporter/internal/constants"
)

var (
	snapshotsPath         = constants.DefaultSnapshotsPath
	snapshotScopes        = []ScopeDefinition{{Name: constants.SnapshotsAppsSubdir, Namespaced: true}}
	snapshotsPVCName      = constants.DefaultSnapshotsPVCName
	snapshotsPVCNamespace = constants.DefaultSnapshotsPVCNamespace
	snapshotsMaxVersions  = constants.DefaultSnapshotsMaxVersions
	snapshotGCInterval    = constants.DefaultSnapshotGCInterval
)

type ScopeDefinition struct {
	Name       string
	Namespaced bool
}

func InitSnapshotsPath() string {
	envPath := strings.TrimSpace(getEnv(constants.SnapshotsPathEnv))
	if envPath != constants.EmptyString {
		snapshotsPath = envPath
	}

	envPVCName := strings.TrimSpace(getEnv(constants.SnapshotsPVCNameEnv))
	if envPVCName != constants.EmptyString {
		snapshotsPVCName = envPVCName
	}

	envPVCNamespace := strings.TrimSpace(getEnv(constants.SnapshotsPVCNamespaceEnv))
	if envPVCNamespace != constants.EmptyString {
		snapshotsPVCNamespace = envPVCNamespace
	}

	return snapshotsPath
}

func InitSnapshotsMaxVersions() int {
	envVal := strings.TrimSpace(getEnv(constants.SnapshotsMaxVersionsEnv))
	if envVal == constants.EmptyString {
		snapshotsMaxVersions = constants.DefaultSnapshotsMaxVersions
		return snapshotsMaxVersions
	}
	n, err := strconv.Atoi(envVal)
	if err != nil || n < constants.MinSnapshotsMaxVersions {
		snapshotsMaxVersions = constants.DefaultSnapshotsMaxVersions
		return snapshotsMaxVersions
	}
	snapshotsMaxVersions = n
	return snapshotsMaxVersions
}

// Zero disables the sweep; a malformed or negative value falls back to the default.
func InitSnapshotGCInterval() time.Duration {
	envVal := strings.TrimSpace(getEnv(constants.SnapshotGCIntervalSecEnv))
	if envVal == constants.EmptyString {
		snapshotGCInterval = constants.DefaultSnapshotGCInterval
		return snapshotGCInterval
	}
	n, err := strconv.Atoi(envVal)
	if err != nil || n < constants.DefaultInitValue {
		snapshotGCInterval = constants.DefaultSnapshotGCInterval
		return snapshotGCInterval
	}
	snapshotGCInterval = time.Duration(n) * time.Second
	return snapshotGCInterval
}

func GetSnapshotGCInterval() time.Duration {
	return snapshotGCInterval
}

func GetSnapshotsPath() string {
	return snapshotsPath
}

func GetSnapshotsScopeRoot(scope string) string {
	return filepath.Join(snapshotsPath, scope)
}

func GetSnapshotScopes() []ScopeDefinition {
	return snapshotScopes
}

func GetSnapshotsMaxVersions() int {
	if snapshotsMaxVersions < constants.MinSnapshotsMaxVersions {
		return constants.DefaultSnapshotsMaxVersions
	}
	return snapshotsMaxVersions
}

func GetSnapshotsPVCName() string {
	return snapshotsPVCName
}

func GetSnapshotsPVCNamespace() string {
	return snapshotsPVCNamespace
}

func getEnv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func IsScopeValid(scopes []ScopeDefinition, name string) bool {
	for _, s := range scopes {
		if s.Name == name {
			return true
		}
	}
	return false
}

func IsScopeNamespaced(scopes []ScopeDefinition, name string) bool {
	for _, s := range scopes {
		if s.Name == name {
			return s.Namespaced
		}
	}
	return true
}
