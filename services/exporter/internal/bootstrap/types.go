package bootstrap

import (
	"github.com/telark/telark/internal/data/resources/telarkconfig"
	eps "github.com/telark/telark/internal/rest/endpoints/bootstrap"
)

// Changed holds the requests a call moved to another state, and Reissued says a completed one gave its token again,
// for the notices; Pinned names the pinned account, which hears of them beside every active Admin on ALL.
type Outcome struct {
	Result   eps.RequestResult
	Pinned   string
	Changed  []telarkconfig.BootstrapRequest
	Reissued bool
}
