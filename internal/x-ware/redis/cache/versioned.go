package cache

import (
	"encoding/json"
	"time"

	"github.com/telark/x-ware/constants"
)

func SetWithVersion(
	set func(key string, val any, ttl time.Duration) error,
	key string,
	value any,
	version string,
	ttl time.Duration,
) error {
	if version == constants.EmptyString {
		return set(key, value, ttl)
	}
	versionedKey := key + KeyDelimiter + version
	if err := set(versionedKey, value, ttl); err != nil {
		return err
	}
	return set(key, value, ttl)
}

func GetWithVersionCheck(
	get func(key string) (string, error),
	key string,
	currentVersion string,
	extractVersion func(raw string) (string, bool),
) (val string, exists bool, stale bool) {
	raw, err := get(key)
	if err != nil || raw == constants.EmptyString {
		return constants.EmptyString, false, false
	}
	if currentVersion == constants.EmptyString || extractVersion == nil {
		return raw, true, false
	}
	if v, ok := extractVersion(raw); ok && v != constants.EmptyString && v != currentVersion {
		return raw, true, true
	}
	return raw, true, false
}

func ExtractResourceVersionJSON(raw string) (string, bool) {
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return constants.EmptyString, false
	}
	meta, ok := obj["metadata"].(map[string]any)
	if !ok {
		return constants.EmptyString, false
	}
	v, ok := meta["resourceVersion"].(string)
	return v, ok
}
