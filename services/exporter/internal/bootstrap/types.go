package bootstrap

import (
	"github.com/telark/telark/internal/data/resources/telarkconfig"
	eps "github.com/telark/telark/internal/rest/endpoints/bootstrap"
)

// Changed holds the requests a call moved to another state, and Issued says break-glass is issuing a link for the request
// (a recovery's, or a completed replacement's again), for the notices; Pinned names the pinned account, which hears of
// them beside every active Admin on ALL.
type Outcome struct {
	Result  eps.RequestResult
	Pinned  string
	Changed []telarkconfig.BootstrapRequest
	Issued  bool
}
