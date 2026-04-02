package lock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// creates a new distributed lock backed by the given Redis client.
func New(client *redis.Client) *Lock {
	return &Lock{client: client}
}

// generates a unique holder identifier in the format
func GenerateHolderID() string {
	host, _ := os.Hostname()
	pid := os.Getpid()
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	return fmt.Sprintf("%s%s%d%s%s", host, HolderSeparator, pid, HolderSeparator, id)
}

func fenceKey(key string) string {
	return key + FenceKeySuffix
}

func (l *Lock) Acquire(ctx context.Context, key, holder string, ttl time.Duration) (string, bool, error) {
	if err := l.validate(key, holder, ttl); err != nil {
		return "", false, err
	}

	var fenceToken string
	var acquired bool

	err := l.client.Watch(ctx, func(tx *redis.Tx) error {
		current, err := tx.Get(ctx, key).Result()
		if err != nil && err != redis.Nil {
			return err
		}

		fk := fenceKey(key)

		if err == redis.Nil {
			// Key does not exist — acquire it
			_, txErr := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, key, holder, ttl)
				pipe.Incr(ctx, fk)
				return nil
			})
			if txErr != nil {
				return txErr
			}

			// Read the fence token after transaction
			token, fErr := l.client.Get(ctx, fk).Result()
			if fErr != nil {
				return fErr
			}
			fenceToken = token
			acquired = true
			return nil
		}

		if current == holder {
			// Reentrant — extend TTL
			_, txErr := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.PExpire(ctx, key, ttl)
				return nil
			})
			if txErr != nil {
				return txErr
			}

			token, fErr := l.client.Get(ctx, fk).Result()
			if fErr != nil && fErr != redis.Nil {
				return fErr
			}
			fenceToken = token
			acquired = true
			return nil
		}

		// Different holder owns the lock
		acquired = false
		return nil
	}, key)

	if err == redis.TxFailedErr {
		// Transaction aborted due to concurrent modification
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf(string(ErrAcquireFailed), err)
	}

	return fenceToken, acquired, nil
}

func (l *Lock) Release(ctx context.Context, key, holder string) (bool, error) {
	if l.client == nil {
		return false, fmt.Errorf("%s", ErrNilRedisClient)
	}
	if key == "" {
		return false, fmt.Errorf("%s", ErrEmptyKey)
	}
	if holder == "" {
		return false, fmt.Errorf("%s", ErrEmptyHolder)
	}

	var released bool

	err := l.client.Watch(ctx, func(tx *redis.Tx) error {
		current, err := tx.Get(ctx, key).Result()
		if err == redis.Nil {
			// Lock does not exist — nothing to release
			released = false
			return nil
		}
		if err != nil {
			return err
		}

		if current != holder {
			released = false
			return nil
		}

		_, txErr := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Del(ctx, key)
			return nil
		})
		if txErr != nil {
			return txErr
		}
		released = true
		return nil
	}, key)

	if err == redis.TxFailedErr {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf(string(ErrReleaseFailed), err)
	}

	return released, nil
}

// extends the lock TTL only if the caller is the current holder.
func (l *Lock) Extend(ctx context.Context, key, holder string, ttl time.Duration) (bool, error) {
	if err := l.validate(key, holder, ttl); err != nil {
		return false, err
	}

	var extended bool

	err := l.client.Watch(ctx, func(tx *redis.Tx) error {
		current, err := tx.Get(ctx, key).Result()
		if err == redis.Nil {
			extended = false
			return nil
		}
		if err != nil {
			return err
		}

		if current != holder {
			extended = false
			return nil
		}

		_, txErr := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.PExpire(ctx, key, ttl)
			return nil
		})
		if txErr != nil {
			return txErr
		}
		extended = true
		return nil
	}, key)

	if err == redis.TxFailedErr {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf(string(ErrExtendFailed), err)
	}

	return extended, nil
}

func (l *Lock) FenceToken(ctx context.Context, key string) (string, error) {
	if l.client == nil {
		return "", fmt.Errorf("%s", ErrNilRedisClient)
	}
	if key == "" {
		return "", fmt.Errorf("%s", ErrEmptyKey)
	}

	token, err := l.client.Get(ctx, fenceKey(key)).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf(string(ErrFenceTokenFailed), err)
	}
	return token, nil
}

func (l *Lock) validate(key, holder string, ttl time.Duration) error {
	if l.client == nil {
		return fmt.Errorf("%s", ErrNilRedisClient)
	}
	if key == "" {
		return fmt.Errorf("%s", ErrEmptyKey)
	}
	if holder == "" {
		return fmt.Errorf("%s", ErrEmptyHolder)
	}
	if ttl <= 0 {
		return fmt.Errorf("%s", ErrZeroTTL)
	}
	return nil
}

func FormatFenceToken(val int64) string {
	return strconv.FormatInt(val, 10)
}
