package snapshot

import (
	"fmt"

	"github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/services/discovery/internal/config"
	"github.com/telark/telark/services/discovery/internal/constants"
)

type DeleteSnapshotFn func(id string, scope string, namespace string, generation int) error

// Retention only globs within a single snapshot id's directory, so a file that
// was never recorded is never reclaimed.
func DiscardSnapshots(snaps []application.ApplicationSnapshot, del DeleteSnapshotFn) {
	if del == nil {
		return
	}
	scope := config.DefaultSnapshotScope()
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	for i := range snaps {
		s := snaps[i]
		if err := del(s.ID, scope, s.Namespace, s.Generation); err != nil {
			lg.Warn(fmt.Sprintf(
				string(constants.ErrOrphanSnapshotNotRemoved),
				s.ID,
				s.Namespace,
				s.Generation,
				err,
			))
		}
	}
}
