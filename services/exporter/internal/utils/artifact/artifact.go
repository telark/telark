package artifact

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/internal/x-ware/redis/stream"
	"github.com/telark/telark/services/exporter/internal/constants"
)

const baseSeparatorShift = 1

// The temp file is uniquely named per write. A fixed name is truncated by every
// concurrent writer of the same path, so one of them renames a half-written
// file into place. The rename itself is atomic, so readers only ever see a
// complete file and the last writer wins. The error is the raw cause of the
// failing call; the stage tells the caller which one it was.
func WriteAtomic(path string, write func(io.Writer) error) (Stage, error) {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+constants.SnapshotTempSuffix)
	if err != nil {
		return StageTempCreate, err
	}
	tmpPath := f.Name()
	if err := write(f); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return StageTempWrite, err
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return StageTempClose, err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return StageRename, err
	}

	return StageNone, nil
}

// Every start of a migrating pod copies again, two pods may copy at once, and a
// claim that returns after a rollback holds newer files: the newer side wins.
func CopyTree(src string, dst string) (int, error) {
	copied := constants.DefaultInitValue
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() && d.Name() == constants.LostFoundDir {
			return fs.SkipDir
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, constants.SnapshotDirPerm)
		}
		if !d.Type().IsRegular() || strings.HasSuffix(d.Name(), constants.SnapshotTempFileSuffix) {
			return nil
		}
		done, err := copyIfNewer(path, target, d)
		if done {
			copied++
		}
		return err
	})
	return copied, err
}

func copyIfNewer(path string, target string, d fs.DirEntry) (bool, error) {
	info, err := d.Info()
	if err != nil {
		return false, err
	}
	if existing, statErr := os.Stat(target); statErr == nil && !info.ModTime().After(existing.ModTime()) {
		return false, nil
	}
	src, err := os.Open(filepath.Clean(path))
	if err != nil {
		return false, err
	}
	defer func() { _ = src.Close() }()
	_, err = WriteAtomic(target, func(w io.Writer) error {
		_, copyErr := io.Copy(w, src)
		return copyErr
	})
	if err != nil {
		return false, err
	}
	return true, os.Chtimes(target, info.ModTime(), info.ModTime())
}

func IsWithinBase(targetPath string, basePath string) bool {
	return len(targetPath) > len(basePath) &&
		targetPath[:len(basePath)+baseSeparatorShift] == basePath+string(os.PathSeparator)
}

func IsSafeSegment(id string) bool {
	clean := filepath.Base(id)
	return id != constants.EmptyString && clean == id && clean != "." && clean != ".."
}

// The lock only spares the second replica a periodic sweep: a Redis outage runs
// the sweep rather than skipping it. A canceled context is shutdown, not a
// Redis error, so it never sweeps.
func TickAllowed(ctx context.Context, rdb *redis.Client, key string, ttl time.Duration) bool {
	if ctx.Err() != nil {
		return false
	}
	if rdb == nil {
		return true
	}
	hostname, _ := os.Hostname()
	acquired, err := stream.NewLockClient(rdb).Acquire(ctx, key, hostname, ttl)
	if ctx.Err() != nil {
		return false
	}
	return err != nil || acquired
}
