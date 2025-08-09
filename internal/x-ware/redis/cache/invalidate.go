package cache

import (
	"context"
	"strings"
)

func IsAction(action string, actions []string) bool {
	a := strings.ToLower(action)
	for _, x := range actions {
		if strings.Contains(a, strings.ToLower(x)) {
			return true
		}
	}
	return false
}

func IsModifyingAction(action string, modifyingActions []string) bool {
	return IsAction(action, modifyingActions)
}

func DeleteKey(delete func(key string) error, key string) error {
	return delete(key)
}

func DeleteKeys(ctx context.Context, deleter func(ctx context.Context, keys ...string) (int64, error), keys ...string) (int64, error) {
	if len(keys) == 0 {
		return 0, nil
	}
	return deleter(ctx, keys...)
}

func DeleteByPattern(ctx context.Context, scanner Scanner, deleter func(ctx context.Context, keys ...string) (int64, error), pattern string, batch int64) (int, error) {
	var (
		cursor   uint64
		totalDel int
	)
	if batch <= 0 {
		batch = 1000
	}
	for {
		keys, next, err := scanner.Scan(ctx, cursor, pattern, batch)
		if err != nil {
			return totalDel, err
		}
		if len(keys) > 0 {
			n, err := deleter(ctx, keys...)
			if err != nil {
				return totalDel, err
			}
			totalDel += int(n)
		}
		if next == 0 {
			break
		}
		cursor = next
	}
	return totalDel, nil
}
