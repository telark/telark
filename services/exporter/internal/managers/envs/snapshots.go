package envs

import (
	"os"
	"path/filepath"
	"slices"
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
	snapshotStatsRefresh  = constants.DefaultSnapshotStatsRefreshInterval
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

// Zero disables the cache and walks the volume per request; a malformed or
// negative value falls back to the default.
func InitSnapshotStatsRefreshInterval() time.Duration {
	envVal := strings.TrimSpace(getEnv(constants.SnapshotStatsRefreshSecEnv))
	if envVal == constants.EmptyString {
		snapshotStatsRefresh = constants.DefaultSnapshotStatsRefreshInterval
		return snapshotStatsRefresh
	}
	n, err := strconv.Atoi(envVal)
	if err != nil || n < constants.DefaultInitValue {
		snapshotStatsRefresh = constants.DefaultSnapshotStatsRefreshInterval
		return snapshotStatsRefresh
	}
	snapshotStatsRefresh = time.Duration(n) * time.Second
	return snapshotStatsRefresh
}

func GetSnapshotStatsRefreshInterval() time.Duration {
	return snapshotStatsRefresh
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

func scopeIndex(scopes []ScopeDefinition, name string) int {
	return slices.IndexFunc(scopes, func(s ScopeDefinition) bool { return s.Name == name })
}

func IsScopeValid(scopes []ScopeDefinition, name string) bool {
	return scopeIndex(scopes, name) >= constants.DefaultInitValue
}

// An unregistered scope is treated as namespaced: the caller then has to supply
// a namespace instead of silently writing to the scope root.
func IsScopeNamespaced(scopes []ScopeDefinition, name string) bool {
	if i := scopeIndex(scopes, name); i >= constants.DefaultInitValue {
		return scopes[i].Namespaced
	}
	return true
}
