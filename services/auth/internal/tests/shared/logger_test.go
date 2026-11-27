package shared

import (
	"io"
	"os"
	"regexp"
	"testing"

	"github.com/telark/telark/internal/data/logger"
	"github.com/telark/telark/services/auth/internal/constants"
)

const loggedMessage = "ready"

// The shared loggers' shape, "Informers: 2026/11/23 10:00:00 [INFO] ...".
var sharedLine = regexp.MustCompile(`^[A-Z][A-Za-z]*: \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} \[INFO\] ` + loggedMessage + "\n$")

// Seen live: every auth line read "auth-request2026/11/23 ...", the prefix glued to the date.
func TestLoggerPrefixesReadLikeTheSharedLoggers(t *testing.T) {
	for _, prefix := range []string{
		constants.LoggerPrefixAuthService, constants.LoggerPrefixHandler, constants.LoggerPrefixHelper,
		constants.LoggerPrefixOIDC, constants.LoggerPrefixRedis, constants.LoggerPrefixAsync,
		constants.LoggerPrefixCleanup, constants.LoggerPrefixAuthRequest,
	} {
		if line := loggedLine(t, prefix); !sharedLine.MatchString(line) {
			t.Errorf("prefix %q logs %q, want the shared shape %q", prefix, line, sharedLine)
		}
	}
}

// The logger binds os.Stdout when it is built, so only the build runs with stdout swapped.
func loggedLine(t *testing.T, prefix string) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = writer
	lg := logger.NewCustomLogger(prefix)
	os.Stdout = stdout
	lg.Info(loggedMessage)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
