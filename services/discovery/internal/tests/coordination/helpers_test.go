package coordination

import (
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/coordination"
	"github.com/telark/discovery/internal/tests/testutil"
)

// The stream payload carries all six message fields, with attempts stringified.
func TestGenerateMsgPayload(t *testing.T) {
	payload := coordination.GenerateMsgPayload("app", "ns", "cyc", "discover", "2026-01-01T00:00:00Z", 3)
	testutil.Equal(t, "field count", len(payload), 6)
	testutil.Equal(t, "app name", payload[constants.StreamMsgFieldAppName], any("app"))
	testutil.Equal(t, "attempts stringified", payload[constants.StreamMsgFieldAttempts], any("3"))
}

// MsgField reads a string value, and returns empty for a missing key or a
// non-string value.
func TestMsgField(t *testing.T) {
	msg := redis.XMessage{Values: map[string]any{"k": "v", "n": 7}}
	testutil.Equal(t, "present", coordination.MsgField(msg, "k"), "v")
	testutil.Equal(t, "missing", coordination.MsgField(msg, "absent"), "")
	testutil.Equal(t, "non-string", coordination.MsgField(msg, "n"), "")
}
