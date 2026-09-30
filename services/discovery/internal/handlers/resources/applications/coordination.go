package applications

import (
	"sync"

	"github.com/telark/telark/services/discovery/internal/coordination"
	"github.com/telark/telark/services/discovery/internal/coordination/forcesync"
)

var (
	coordBundle      *coordination.CoordinationBundle
	coordReplica     string
	coordMu          sync.RWMutex
	forceSyncIngress *forcesync.Ingress
	ingressMu        sync.RWMutex
)

func SetCoordinationBundle(bundle *coordination.CoordinationBundle, replicaID string) {
	coordMu.Lock()
	defer coordMu.Unlock()
	coordBundle = bundle
	coordReplica = replicaID
}

func getCoordinationBundle() (*coordination.CoordinationBundle, string) {
	coordMu.RLock()
	defer coordMu.RUnlock()
	return coordBundle, coordReplica
}

func SetForceSyncIngress(ingress *forcesync.Ingress) {
	ingressMu.Lock()
	defer ingressMu.Unlock()
	forceSyncIngress = ingress
}

func getForceSyncIngress() *forcesync.Ingress {
	ingressMu.RLock()
	defer ingressMu.RUnlock()
	return forceSyncIngress
}
