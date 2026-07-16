package shared

import (
	"net/http"
	"os"
	"strings"
	"sync"

	dataconstants "github.com/telark/data/constants"
	"github.com/telark/rest/constants"
)

var (
	serviceTokenOnce  sync.Once
	serviceTokenValue string
)

func serviceToken() string {
	serviceTokenOnce.Do(func() {
		serviceTokenValue = strings.TrimSpace(os.Getenv(dataconstants.EnvServiceToken))
	})
	return serviceTokenValue
}

// applyServiceToken identifies this process to the service it calls, so the
// receiver authorizes the caller instead of trusting its network position.
// Absent a token the request goes out unidentified and is refused there, which
// is the intended outcome: a caller that cannot prove itself gets no access.
func applyServiceToken(req *http.Request) {
	token := serviceToken()
	if token == constants.EmptyString {
		return
	}
	req.Header.Set(dataconstants.HeaderServiceToken, token)
}
