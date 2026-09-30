package snapshot

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/telark/telark/services/exporter/internal/constants"
)

func parseGenerationFilename(name string) (int, bool) {
	if !strings.HasPrefix(name, constants.SnapshotFilePrefix) || !strings.HasSuffix(name, constants.SnapshotFileExtension) {
		return constants.DefaultInitValue, false
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(name, constants.SnapshotFilePrefix), constants.SnapshotFileExtension)
	if raw == constants.EmptyString {
		return constants.DefaultInitValue, false
	}
	for _, r := range raw {
		if r < '0' || r > '9' {
			return constants.DefaultInitValue, false
		}
	}
	v, err := strconv.Atoi(raw)
	return v, err == nil
}

func resolveLatestInDir(dir string) (int, string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "V*.json"))
	if err != nil {
		return constants.DefaultInitValue, constants.EmptyString, err
	}
	bestGen := constants.DefaultInitValue
	bestPath := constants.EmptyString
	for _, file := range files {
		gen, ok := parseGenerationFilename(filepath.Base(file))
		if !ok {
			continue
		}
		if gen > bestGen {
			bestGen = gen
			bestPath = file
		}
	}
	if bestPath == constants.EmptyString {
		return constants.DefaultInitValue, constants.EmptyString, ErrLatestNotFound
	}
	return bestGen, bestPath, nil
}

func ResolveSnapshotPath(
	snapshotsPath string,
	scope string,
	id string,
	namespace string,
	generationParam string,
	namespaced bool,
) (*SnapshotTarget, error) {
	generationParam = strings.TrimSpace(generationParam)
	if generationParam == constants.EmptyString {
		generationParam = latestGenerationValue
	}
	if namespaced {
		if namespace != constants.EmptyString {
			return resolveInNamespace(snapshotsPath, scope, id, namespace, generationParam)
		}
		return resolveAcrossNamespaces(snapshotsPath, scope, id, generationParam)
	}
	return resolveInNamespace(snapshotsPath, scope, id, constants.EmptyString, generationParam)
}

func resolveInNamespace(
	snapshotsPath string,
	scope string,
	id string,
	namespace string,
	generationParam string,
) (*SnapshotTarget, error) {
	namespaced := namespace != constants.EmptyString
	dir := BuildSnapshotDir(snapshotsPath, scope, id, namespace, namespaced)
	if generationParam == latestGenerationValue {
		gen, path, err := resolveLatestInDir(dir)
		if err != nil {
			return nil, err
		}
		return &SnapshotTarget{Path: path, Namespace: namespace, Generation: gen}, nil
	}
	gen, err := strconv.Atoi(generationParam)
	if err != nil || gen < constants.DefaultIncrementValue {
		return nil, errors.New(string(constants.ErrSnapshotGenerationRequired))
	}
	path := BuildSnapshotPath(snapshotsPath, scope, id, namespace, gen, namespaced)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrLatestNotFound
		}
		return nil, err
	}
	return &SnapshotTarget{Path: path, Namespace: namespace, Generation: gen}, nil
}

func resolveAcrossNamespaces(
	snapshotsPath string,
	scope string,
	id string,
	generationParam string,
) (*SnapshotTarget, error) {
	baseDir := filepath.Join(snapshotsPath, scope, id)
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrLatestNotFound
		}
		return nil, err
	}
	best := &SnapshotTarget{Generation: constants.DefaultInitValue}
	targetGeneration := constants.DefaultInitValue
	if generationParam != latestGenerationValue {
		targetGeneration, err = strconv.Atoi(generationParam)
		if err != nil || targetGeneration < constants.DefaultIncrementValue {
			return nil, errors.New(string(constants.ErrSnapshotGenerationRequired))
		}
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		namespace := entry.Name()
		dir := filepath.Join(baseDir, namespace)
		if generationParam == latestGenerationValue {
			updateBestFromLatest(best, dir, namespace)
			continue
		}
		updateBestFromExplicitGeneration(best, snapshotsPath, scope, id, namespace, targetGeneration)
	}
	if best.Path == constants.EmptyString {
		return nil, ErrLatestNotFound
	}
	return best, nil
}

func updateBestFromLatest(best *SnapshotTarget, dir string, namespace string) {
	gen, path, latestErr := resolveLatestInDir(dir)
	if latestErr != nil {
		return
	}
	if gen > best.Generation {
		best.Generation = gen
		best.Path = path
		best.Namespace = namespace
	}
}

func updateBestFromExplicitGeneration(
	best *SnapshotTarget,
	snapshotsPath string,
	scope string,
	id string,
	namespace string,
	targetGeneration int,
) {
	path := BuildSnapshotPath(snapshotsPath, scope, id, namespace, targetGeneration, true)
	if _, statErr := os.Stat(path); statErr == nil && targetGeneration >= best.Generation {
		best.Generation = targetGeneration
		best.Path = path
		best.Namespace = namespace
	}
}
