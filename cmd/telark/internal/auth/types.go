package auth

import (
	"io"

	"github.com/telark/telark/cmd/telark/internal/constants"
	"github.com/telark/telark/cmd/telark/internal/kube"
	"github.com/telark/telark/cmd/telark/internal/output"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
)

type options struct {
	kube   *kube.Flags
	email  string
	format output.Format
}

// The private key lives only here, in memory: nothing else can open the sealed link.
type enrollment struct {
	out     io.Writer
	printer *output.Printer
	name    string
	phase   string
	public  *[constants.SealedKeyBytes]byte
	private *[constants.SealedKeyBytes]byte
}

type Status struct {
	Email         string                         `json:"email,omitempty"`
	PinnedAt      string                         `json:"pinnedAt,omitempty"`
	Pinned        bool                           `json:"pinned"`
	NewestRequest *telarkconfig.BootstrapRequest `json:"newestRequest,omitempty"`
}
