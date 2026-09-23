package protection

import (
	"sync/atomic"

	"github.com/telark/discovery/internal/core/plans/protection"
)

// The bootstrap publishes the service from its own goroutine while the HTTP server is
// already serving, and re-publishes it on every bootstrap path, so the pointer is read
// and written concurrently.
var globalService atomic.Pointer[protection.Service]

func InitService(svc *protection.Service) {
	globalService.Store(svc)
}
