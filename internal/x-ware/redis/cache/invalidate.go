package cache

import (
	"context"
	"strings"

	"github.com/telark/x-ware/constants"
)

const defaultScanBatchSize = 1000

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

func DeleteKey(del func(key string) error, key string) error {
	return del(key)
}

func DeleteKeys(
	ctx context.Context,
	deleter func(ctx context.Context, keys ...string) (int64, error),
	keys ...string,
) (int64, error) {
	if len(keys) == constants.EmptySliceLength {
		return constants.ZeroValue, nil
	}
	return deleter(ctx, keys...)
}

func DeleteByPattern(
	ctx context.Context,
	scanner Scanner,
	deleter func(ctx context.Context, keys ...string) (int64, error),
	pattern string,
	batch int64,
) (int, error) {
	var (
		cursor   uint64
		totalDel int
	)
	if batch <= constants.ZeroValue {
		batch = defaultScanBatchSize
	}
	for {
		keys, next, err := scanner.Scan(ctx, cursor, pattern, batch)
		if err != nil {
			return totalDel, err
		}
		if len(keys) > constants.EmptySliceLength {
			n, err := deleter(ctx, keys...)
			if err != nil {
				return totalDel, err
			}
			totalDel += int(n)
		}
		if next == constants.ZeroValue {
			break
		}
		cursor = next
	}
	return totalDel, nil
}
