package coordination

import (
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/coordination"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	streamMsgFieldCount = 6
	nonStringFieldValue = 7
)

// The stream payload carries all six message fields, with attempts stringified.
func TestGenerateMsgPayload(t *testing.T) {
	payload := coordination.GenerateMsgPayload("app", "ns", "cyc", "discover", "2026-01-01T00:00:00Z", constants.ThreeValue)
	testutil.Equal(t, "field count", len(payload), streamMsgFieldCount)
	testutil.Equal(t, "app name", payload[constants.StreamMsgFieldAppName], any("app"))
	testutil.Equal(t, "attempts stringified", payload[constants.StreamMsgFieldAttempts], any("3"))
}

// MsgField reads a string value, and returns empty for a missing key or a
// non-string value.
func TestMsgField(t *testing.T) {
	msg := redis.XMessage{Values: map[string]any{"k": "v", "n": nonStringFieldValue}}
	testutil.Equal(t, "present", coordination.MsgField(msg, "k"), "v")
	testutil.Equal(t, "missing", coordination.MsgField(msg, "absent"), "")
	testutil.Equal(t, "non-string", coordination.MsgField(msg, "n"), "")
}
