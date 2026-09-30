package oidc

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
)

var (
	trustMu        sync.Mutex
	trustModTime   time.Time
	trustSize      int64
	trustJSON      string
	pinned         bool
	pinnedJSON     string
	pinnedAtModify time.Time
)

// The kubelet refreshes the mounted Secret about a minute after the exporter writes
// it, so a set validated here (or cleared, "") wins until the file's mtime moves past the one it saw.
func PinTrustJWK(jwkJSON string) {
	trustMu.Lock()
	defer trustMu.Unlock()
	pinned, pinnedJSON = true, jwkJSON
	pinnedAtModify = statModTime()
}

func TrustJWK() string {
	trustMu.Lock()
	defer trustMu.Unlock()

	info, err := os.Stat(config.OIDCTrustFile())
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			lg.Warn(fmt.Sprintf(string(constants.ErrOIDCTrustFileRead), err))
		}
		trustModTime, trustSize, trustJSON = time.Time{}, constants.DefaultInitValue, constants.EmptyString
		return pinnedJSON
	}

	if !info.ModTime().Equal(trustModTime) || info.Size() != trustSize {
		raw, readErr := os.ReadFile(config.OIDCTrustFile())
		if readErr != nil {
			lg.Warn(fmt.Sprintf(string(constants.ErrOIDCTrustFileRead), readErr))
			return pinnedJSON
		}
		trustModTime, trustSize, trustJSON = info.ModTime(), info.Size(), strings.TrimSpace(string(raw))
	}

	if pinned {
		if trustModTime.Equal(pinnedAtModify) {
			return pinnedJSON
		}
		pinned, pinnedJSON = false, constants.EmptyString
	}
	return trustJSON
}

func statModTime() time.Time {
	info, err := os.Stat(config.OIDCTrustFile())
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
