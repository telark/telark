package redis

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	goredis "github.com/redis/go-redis/v9"
	"github.com/telark/x-ware/redis/cache"
	"github.com/telark/x-ware/redis/stream"
)

const (
	unreachableAddr = "127.0.0.1:1"
	opID            = "connectivity:service:exporter"
	statePrefix     = "operation:"
	cachePrefix     = "exporter-cache:"
	cmdSet          = "set"
	cmdScan         = "scan"
	cmdDel          = "del"
	cmdFlushDB      = "flushdb"
	scanEnd         = 0
	none            = 0
	one             = 1
	keyArg          = 1
)

type recordingHook struct {
	mu       sync.Mutex
	commands [][]string
	scanKeys []string
}

func (*recordingHook) DialHook(next goredis.DialHook) goredis.DialHook { return next }

func (*recordingHook) ProcessPipelineHook(next goredis.ProcessPipelineHook) goredis.ProcessPipelineHook {
	return next
}

func (h *recordingHook) ProcessHook(goredis.ProcessHook) goredis.ProcessHook {
	return func(_ context.Context, cmd goredis.Cmder) error {
		args := make([]string, len(cmd.Args()))
		for i, arg := range cmd.Args() {
			args[i] = fmt.Sprint(arg)
		}
		h.mu.Lock()
		h.commands = append(h.commands, args)
		h.mu.Unlock()
		if scan, ok := cmd.(*goredis.ScanCmd); ok {
			scan.SetVal(h.scanKeys, scanEnd)
		}
		return nil
	}
}

func (h *recordingHook) named(name string) [][]string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out [][]string
	for _, c := range h.commands {
		if strings.EqualFold(c[0], name) {
			out = append(out, c)
		}
	}
	return out
}

func hookedClient(t *testing.T, hook *recordingHook) *goredis.Client {
	t.Helper()
	client := goredis.NewClient(&goredis.Options{Addr: unreachableAddr})
	client.AddHook(hook)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestOperationStateKeysAreNamespaced(t *testing.T) {
	hook := &recordingHook{}
	state := stream.NewStateClient(hookedClient(t, hook))

	if err := state.Set(context.Background(), stream.OperationState{ID: opID}); err != nil {
		t.Fatalf("set: %v", err)
	}
	sets := hook.named(cmdSet)
	if len(sets) != one || sets[none][keyArg] != statePrefix+opID {
		t.Fatalf("SET commands = %v, want key %q", sets, statePrefix+opID)
	}
}

func TestCacheFlushIsScopedToItsPrefix(t *testing.T) {
	cases := []struct {
		name     string
		prefix   string
		wantErr  bool
		wantScan string
	}{
		{name: "no prefix refuses", prefix: "", wantErr: true},
		{name: "prefix deletes only its keys", prefix: cachePrefix, wantScan: cachePrefix + "*"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hook := &recordingHook{scanKeys: []string{tc.prefix + "a"}}
			c := &cache.RedisCache{Client: hookedClient(t, hook), KeyPrefix: tc.prefix}

			err := c.Flush(context.Background())
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if got := hook.named(cmdFlushDB); len(got) != none {
				t.Fatalf("FLUSHDB must never be sent, got %v", got)
			}
			scans := hook.named(cmdScan)
			if tc.wantScan == "" {
				if len(scans) != none || len(hook.named(cmdDel)) != none {
					t.Fatalf("unscoped flush touched keys: %v", hook.commands)
				}
				return
			}
			if len(scans) != one || !strings.Contains(strings.Join(scans[none], " "), tc.wantScan) {
				t.Fatalf("SCAN = %v, want match %q", scans, tc.wantScan)
			}
			if dels := hook.named(cmdDel); len(dels) != one || dels[none][keyArg] != tc.prefix+"a" {
				t.Fatalf("DEL = %v", dels)
			}
		})
	}
}
