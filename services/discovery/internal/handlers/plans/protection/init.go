package protection

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/telark/data/messages"
	plansmeta "github.com/telark/data/metadata/v1alpha1"

	"github.com/telark/discovery/internal/coordination"
	"github.com/telark/discovery/internal/core/plans/protection"
)

// The bootstrap goroutine publishes and re-publishes the service while the HTTP server
// is already serving, so the pointer is read and written concurrently.
var globalService atomic.Pointer[protection.Service]

var coordBundle atomic.Pointer[coordination.CoordinationBundle]

func InitService(svc *protection.Service) {
	globalService.Store(svc)
}

func SetCoordinationBundle(bundle *coordination.CoordinationBundle) {
	coordBundle.Store(bundle)
}

func getCoordinationBundle() *coordination.CoordinationBundle {
	return coordBundle.Load()
}

func planMessage(msg messages.Message, planID string) string {
	return fmt.Sprintf(string(msg), planID, plansmeta.ProtectionPlanMetadata.Kind)
}

// Detached from cancellation so a destructive sequence finishes if the caller disconnects, while
// keeping the verified identity the plan rules read.
func detached(r *http.Request) context.Context {
	return context.WithoutCancel(r.Context())
}
