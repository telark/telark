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

// The receiver authorizes the caller by this token, not by network position;
// without one the request goes out unidentified and is refused there, on purpose.
func applyServiceToken(req *http.Request) {
	token := serviceToken()
	if token == constants.EmptyString {
		return
	}
	req.Header.Set(dataconstants.HeaderServiceToken, token)
}
