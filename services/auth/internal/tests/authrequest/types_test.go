package authrequest

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	authdata "github.com/telark/telark/internal/data/auth"
	requestdata "github.com/telark/telark/internal/data/resources/authrequest"
	userresource "github.com/telark/telark/internal/data/resources/user"
	"github.com/telark/telark/services/auth/internal/constants"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

// One bubble's request loop: the fake exporter behind it, the requester's key pair and the calls outside the
// AuthRequest routes.
type bench struct {
	fake    *testutil.FakeExporter
	backend http.Handler
	admin   string
	public  *[constants.AuthRequestKeyBytes]byte
	private *[constants.AuthRequestKeyBytes]byte
	cancel  context.CancelFunc
	done    chan struct{}
	others  atomic.Int64
}

// How a request ends: every phase reported for it, in order, and the last one's reason.
type ending struct {
	phases []string
	reason string
}

// At each completion the exporter receives: whether the pinned account was frozen, and when.
type completions struct {
	frozen []bool
	at     []time.Time
}

// Meets the next reads of path, as many as left: runs then, and fails the read unless pass is set.
type intercept struct {
	next http.Handler
	path string
	then func()
	pass bool
	left atomic.Int32
}

type silentCase struct {
	name    string
	users   map[string]*userresource.User
	request func(b *bench) requestdata.AuthRequest
	want    ending
}

type blockedCase struct {
	name    string
	block   func()
	release func()
	lock    string
}

type failedListCase struct {
	name   string
	failed int
	reads  int
}

type shutdownCase struct {
	name   string
	fake   func() *testutil.FakeExporter
	until  string
	lapse  bool
	frozen bool
	phases []string
}

type enrollCase struct {
	name   string
	origin string
	email  string
	user   *userresource.User
	owner  string
}

type replacedLinkCase struct {
	name     string
	marked   bool
	passkeys []authdata.Passkey
	requests []string
	notices  []string
}

type failedStepCase struct {
	name    string
	user    *userresource.User
	backend func(fake *testutil.FakeExporter) http.Handler
	reason  string
}

type waitingCase struct {
	name     string
	origin   string
	hours    int
	pinnedAs string
	kind     string
	target   string
	want     ending
}

type replacementCase struct {
	name      string
	enrolled  bool
	registers bool
	want      ending
}

type stopCase struct {
	name    string
	email   string
	backend func(fake *testutil.FakeExporter) http.Handler
	want    ending
	calls   []string
	lock    string
	frozen  string
}

type unusedCase struct {
	name string
	end  func(t *testing.T, fake *testutil.FakeExporter, outage *intercept)
	want ending
}

type afterLinkCase struct {
	name     string
	path     string
	reads    int32
	pass     bool
	takeover bool
	act      func(t *testing.T, token string)
	reason   string
	frozen   bool
	lock     string
}
