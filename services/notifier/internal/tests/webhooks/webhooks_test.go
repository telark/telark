package webhooks

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	dataconstants "github.com/telark/telark/internal/data/constants"
	notifclient "github.com/telark/telark/internal/rest/clients/notifications"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/notifier/internal/webhooks"
)

const (
	// The Standard Webhooks specification's example secret, message and signature.
	specSecret    = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"
	specID        = "msg_p5jXN8AQM9LWM0D4loKWxJek"
	specTimestamp = "1614265330"
	specBody      = `{"test": 2432232314}`
	specSignature = "v1,g0hM9SsE+OTPJTGt/tmIKtSyZlE3uFJELVlNIOLJ1OE="

	secretPrefix   = "whsec_"
	previousSecret = "whsec_c2Vjb25kLXNlY3JldC1rZXktZm9yLXJvdGF0aW9uLTMy"
	shortSecret    = "whsec_c2hvcnQ="
	// 23 bytes, 31 base64 characters: one short of the minimum; 24 bytes is the minimum.
	secret31Chars = "whsec_AAECAwQFBgcICQoLDA0ODxAREhMUFRY="
	secret32Chars = "whsec_AAECAwQFBgcICQoLDA0ODxAREhMUFRYX"
	serviceToken  = "svc-token"
	consumer      = "test-replica"
	groupPrefix   = "webhook:"
	approval      = notifclient.TypePlanApprovalRequested
	rollback      = notifclient.TypeRollbackCompleted
	eventTitle    = "Protection plan awaiting approval"
	eventMessage  = "**Data freeze** was submitted for approval by admin."
	eventSeverity = "info"
	eventMetadata = `{"planId":"p-1","planName":"Data freeze","targetId":"p-1"}`
	eventPlanName = "Data freeze"
	eventMillis   = int64(1792143000123)
	eventStamp    = "2026-10-16T09:30:00.123Z"
	startUnix     = int64(1792141200)
	opsName       = "ops"
	nameA         = "a"
	nameB         = "b"
	urlA          = "https://a.example.com/"
	urlB          = "https://b.example.com/"
	hookPath      = "/hook"
	firstAttempt  = "attempt 1"
	theAck        = "the ack"
	httpScheme    = "http://"
	httpsScheme   = "https://"
	loopbackIP    = "127.0.0.1"
	portSeparator = ":"
	filePerm      = 0o600
	decimal       = 10
	int64Bits     = 64

	reload         = 20 * time.Millisecond
	tick           = 10 * time.Millisecond
	waitFor        = 3 * time.Second
	quietFor       = 150 * time.Millisecond
	clockSkew      = 5 * time.Second
	afterLastRetry = 10 * time.Hour
	retryAfterWait = time.Hour
	pastFirstDelay = time.Minute
	earlyPercent   = 85
	latePercent    = 115
	percent        = 100

	none          = 0
	one           = 1
	two           = 2
	maxAttempts   = 8
	payloadV1     = 1
	statusOptions = 8
	pendingPage   = 100

	notJSON      = "not an event"
	outage       = "LOADING Redis is loading the dataset in memory"
	noGroupError = "NOGROUP Redis lost its data"
	retryCount   = "RETRYCOUNT"
	noIdle       = 0
)

var retryDelays = []time.Duration{
	30 * time.Second, 2 * time.Minute, 10 * time.Minute, 30 * time.Minute, time.Hour, 2 * time.Hour, 4 * time.Hour,
}

type request struct {
	path   string
	header http.Header
	body   []byte
}

type receiver struct {
	url      string
	mu       sync.Mutex
	requests []request
}

type env struct {
	mr   *miniredis.Miniredis
	rdb  *redis.Client
	path string
}

type deliveredData struct {
	Severity string         `json:"severity"`
	Title    string         `json:"title"`
	Message  string         `json:"message"`
	Metadata map[string]any `json:"metadata"`
}

type deliveredBody struct {
	Version   int           `json:"version"`
	ID        string        `json:"id"`
	Type      string        `json:"type"`
	Timestamp string        `json:"timestamp"`
	Data      deliveredData `json:"data"`
}

func (r *receiver) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.requests)
}

func (r *receiver) last() request {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.requests[len(r.requests)-one]
}

// Loopback is refused even with allowPrivate, so a receiver that must be reached listens on a
// private interface address.
func privateIP(t *testing.T) string {
	t.Helper()
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Skipf("no interface addresses: %v", err)
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil && ipNet.IP.IsPrivate() {
			return ipNet.IP.String()
		}
	}
	t.Skip("no private IPv4 interface address to run a receiver on")
	return ""
}

// answer returns the status of the nth request, from 1, and may set headers.
func newReceiver(t *testing.T, host string, answer func(n int, w http.ResponseWriter) int) *receiver {
	t.Helper()
	recv := &receiver{}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		recv.mu.Lock()
		recv.requests = append(recv.requests, request{path: r.URL.Path, header: r.Header.Clone(), body: body})
		n := len(recv.requests)
		recv.mu.Unlock()
		w.WriteHeader(answer(n, w))
	}))
	listener, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		t.Fatal(err)
	}
	srv.Listener = listener
	srv.Start()
	t.Cleanup(srv.Close)
	recv.url = srv.URL
	return recv
}

func always(status int) func(int, http.ResponseWriter) int {
	return func(int, http.ResponseWriter) int { return status }
}

func newEnv(t *testing.T) env {
	t.Helper()
	t.Setenv(dataconstants.EnvServiceToken, serviceToken)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return env{mr: mr, rdb: rdb, path: filepath.Join(t.TempDir(), "targets.json")}
}

// Encoded as maps, the way the chart renders them, so no struct with a secret field is marshaled.
func encode(t *testing.T, targets []webhooks.TargetConfig) []byte {
	t.Helper()
	entries := make([]map[string]any, none, len(targets))
	for _, config := range targets {
		entries = append(entries, map[string]any{
			"name": config.Name, "url": config.URL, "secret": config.Secret, "previousSecret": config.PreviousSecret,
			"events": config.Events, "allowPrivate": config.AllowPrivate,
		})
	}
	raw, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func (e env) writeRaw(t *testing.T, raw []byte) {
	t.Helper()
	if err := os.WriteFile(e.path, raw, filePerm); err != nil {
		t.Fatal(err)
	}
}

func (e env) run(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		webhooks.NewService(e.rdb, e.path, consumer, reload, tick, tick).Run(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
}

func (e env) groups() []string {
	infos, err := e.rdb.XInfoGroups(context.Background(), notifclient.EventsStream).Result()
	if err != nil {
		return nil
	}
	names := make([]string, none, len(infos))
	for _, info := range infos {
		names = append(names, info.Name)
	}
	return names
}

func (e env) waitGroups(t *testing.T, want ...string) {
	t.Helper()
	waitUntil(t, fmt.Sprintf("groups %v", want), func() bool {
		got := e.groups()
		return len(got) == len(want) && !slices.ContainsFunc(want, func(name string) bool { return !slices.Contains(got, groupPrefix+name) })
	})
}

// Signed the way the exporter signs it.
func (e env) publish(t *testing.T, eventType string) string {
	t.Helper()
	var metadata map[string]any
	if err := json.Unmarshal([]byte(eventMetadata), &metadata); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(notifclient.QueuedEvent{
		Type: eventType, Timestamp: eventMillis, Severity: eventSeverity, Title: eventTitle, Message: eventMessage, Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := xauthz.SignRedisEntry(notifclient.EventsStream, string(payload))
	if !ok {
		t.Fatal("no service token to sign with")
	}
	return e.publishRaw(t, entry)
}

func (e env) publishRaw(t *testing.T, entry string) string {
	t.Helper()
	id, err := e.rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: notifclient.EventsStream,
		Values: []any{notifclient.EventField, entry},
	}).Result()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// The full list, not the summary: miniredis leaves a deleted entry out of the summary only.
func (e env) pending(t *testing.T, name string) int64 {
	t.Helper()
	entries, err := e.rdb.XPendingExt(context.Background(), &redis.XPendingExtArgs{
		Stream: notifclient.EventsStream, Group: groupPrefix + name, Start: "-", End: "+", Count: pendingPage,
	}).Result()
	if err != nil {
		t.Fatal(err)
	}
	return int64(len(entries))
}

// Sets up one running target that answers with answer, ready to receive events.
func (e env) serve(t *testing.T, answer func(int, http.ResponseWriter) int) *receiver {
	t.Helper()
	recv := newReceiver(t, privateIP(t), answer)
	e.writeRaw(t, encode(t, []webhooks.TargetConfig{target(opsName, recv.url, approval)}))
	e.run(t)
	e.waitGroups(t, opsName)
	return recv
}

func waitUntil(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(waitFor)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(tick)
	}
}

func target(name, url string, events ...string) webhooks.TargetConfig {
	return webhooks.TargetConfig{Name: name, URL: url, Secret: specSecret, Events: events, AllowPrivate: true}
}

// Written from the specification, and itself checked against the specification's example.
func expectedSignature(secret, id, timestamp string, body []byte) string {
	key, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, secretPrefix))
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(id + "." + timestamp + "."))
	_, _ = mac.Write(body)
	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func TestParseTargets(t *testing.T) {
	valid := webhooks.TargetConfig{
		Name: opsName, URL: "https://hooks.example.com/telark/T000/secret-path", Secret: specSecret, Events: []string{approval},
	}
	edit := func(change func(*webhooks.TargetConfig)) []webhooks.TargetConfig {
		config := valid
		config.Events = slices.Clone(valid.Events)
		change(&config)
		return []webhooks.TargetConfig{config}
	}
	tests := []struct {
		name    string
		targets []webhooks.TargetConfig
		ok      bool
	}{
		{"valid", []webhooks.TargetConfig{valid}, true},
		{"rotating with a previous secret", edit(func(c *webhooks.TargetConfig) { c.PreviousSecret = previousSecret }), true},
		{"cluster http with allowPrivate", edit(func(c *webhooks.TargetConfig) {
			c.URL, c.AllowPrivate = "http://echo.default.svc.cluster.local:8080/", true
		}), true},
		{"public IP literal", edit(func(c *webhooks.TargetConfig) { c.URL = "https://203.0.113.10/hook" }), true},
		{"bad name", edit(func(c *webhooks.TargetConfig) { c.Name = "Ops Team" }), false},
		{"duplicate name", []webhooks.TargetConfig{valid, valid}, false},
		{"no events", edit(func(c *webhooks.TargetConfig) { c.Events = nil }), false},
		{"account activity", edit(func(c *webhooks.TargetConfig) { c.Events = []string{notifclient.TypeRoleChanged} }), false},
		{"plain http", edit(func(c *webhooks.TargetConfig) { c.URL = "http://hooks.example.com/" }), false},
		{"other scheme", edit(func(c *webhooks.TargetConfig) { c.URL = "ftp://hooks.example.com/" }), false},
		{"user info", edit(func(c *webhooks.TargetConfig) { c.URL = "https://user:pass@hooks.example.com/" }), false},
		{"localhost", edit(func(c *webhooks.TargetConfig) { c.URL = "https://localhost/" }), false},
		{"single label", edit(func(c *webhooks.TargetConfig) { c.URL = "https://echo/" }), false},
		{"service name", edit(func(c *webhooks.TargetConfig) { c.URL = "https://echo.default.svc/" }), false},
		{"cluster name", edit(func(c *webhooks.TargetConfig) { c.URL = "https://echo.default.svc.cluster.local./" }), false},
		{"no host", edit(func(c *webhooks.TargetConfig) { c.URL = "https:///hook" }), false},
		{"secret without prefix", edit(func(c *webhooks.TargetConfig) { c.Secret = strings.TrimPrefix(specSecret, secretPrefix) }), false},
		{"secret not base64", edit(func(c *webhooks.TargetConfig) { c.Secret = "whsec_not base64!" }), false},
		{"short secret", edit(func(c *webhooks.TargetConfig) { c.Secret = shortSecret }), false},
		{"one character short", edit(func(c *webhooks.TargetConfig) { c.Secret = secret31Chars }), false},
		{"the minimum secret", edit(func(c *webhooks.TargetConfig) { c.Secret = secret32Chars }), true},
		{"bad previous secret", edit(func(c *webhooks.TargetConfig) { c.PreviousSecret = shortSecret }), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targets, err := webhooks.ParseTargets(encode(t, tt.targets))
			if (err == nil) != tt.ok || (err == nil && len(targets) != len(tt.targets)) {
				t.Fatalf("ParseTargets: %d targets, err %v, want ok=%v", len(targets), err, tt.ok)
			}
			for _, config := range tt.targets {
				if err != nil && (strings.Contains(err.Error(), config.Secret) || strings.Contains(err.Error(), config.URL)) {
					t.Errorf("the refusal quotes a secret or a URL: %v", err)
				}
			}
		})
	}
	unknown := `[{"name":"ops","url":"https://a.example.com","secret":"` + specSecret + `","events":["` + approval + `"],"extra":1}]`
	if _, err := webhooks.ParseTargets([]byte(unknown)); err == nil {
		t.Error("an unknown field was accepted")
	}
}

func TestSignedDelivery(t *testing.T) {
	if got := expectedSignature(specSecret, specID, specTimestamp, []byte(specBody)); got != specSignature {
		t.Fatalf("the test's verifier disagrees with the specification: %s", got)
	}
	e := newEnv(t)
	recv := newReceiver(t, privateIP(t), always(http.StatusNoContent))
	config := target(opsName, recv.url+hookPath, approval)
	config.PreviousSecret = previousSecret
	e.writeRaw(t, encode(t, []webhooks.TargetConfig{config}))
	e.run(t)
	e.waitGroups(t, opsName)
	id := e.publish(t, approval)
	waitUntil(t, "the delivery", func() bool { return recv.count() == one })

	got := recv.last()
	checkHeaders(t, got, id)
	checkPayload(t, got.body, id)
	waitUntil(t, theAck, func() bool { return e.pending(t, opsName) == none })
}

func checkHeaders(t *testing.T, got request, id string) {
	t.Helper()
	if got.header.Get("Content-Type") != "application/json" || got.header.Get("User-Agent") != "telark-webhooks/1" ||
		got.header.Get("webhook-id") != id {
		t.Errorf("headers %v, want JSON, the user agent and webhook-id %s", got.header, id)
	}
	stamp := got.header.Get("webhook-timestamp")
	seconds, err := strconv.ParseInt(stamp, decimal, int64Bits)
	if err != nil || time.Since(time.Unix(seconds, none)).Abs() > clockSkew {
		t.Errorf("webhook-timestamp %q is not the current Unix time", stamp)
	}
	want := []string{expectedSignature(specSecret, id, stamp, got.body), expectedSignature(previousSecret, id, stamp, got.body)}
	if signatures := strings.Split(got.header.Get("webhook-signature"), " "); !slices.Equal(signatures, want) {
		t.Errorf("webhook-signature %v, want one v1 signature per secret, the current one first", signatures)
	}
}

func checkPayload(t *testing.T, raw []byte, id string) {
	t.Helper()
	var body deliveredBody
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	want := deliveredBody{Version: payloadV1, ID: id, Type: approval, Timestamp: eventStamp, Data: deliveredData{
		Severity: eventSeverity, Title: eventTitle, Message: eventMessage, Metadata: body.Data.Metadata,
	}}
	if body.Version != want.Version || body.ID != want.ID || body.Type != want.Type || body.Timestamp != want.Timestamp ||
		body.Data.Severity != want.Data.Severity || body.Data.Title != want.Data.Title || body.Data.Message != want.Data.Message ||
		body.Data.Metadata["planName"] != eventPlanName {
		t.Errorf("payload %s", raw)
	}
}

// A target gets only the types it lists, one delivery each, and nothing from before it existed.
func TestEventsAndBacklog(t *testing.T) {
	e := newEnv(t)
	e.publish(t, approval)
	recv := e.serve(t, always(http.StatusOK))
	e.publish(t, rollback)
	e.publish(t, approval)
	waitUntil(t, "the delivery", func() bool { return recv.count() == one })
	time.Sleep(quietFor)
	if recv.count() != one {
		t.Errorf("%d deliveries, want 1 (no backlog, no unsubscribed type)", recv.count())
	}
	waitUntil(t, "both entries acked", func() bool { return e.pending(t, opsName) == none })
}

func TestStatusOutcomes(t *testing.T) {
	e := newEnv(t)
	ip := privateIP(t)
	retried := map[int]bool{
		http.StatusOK: false, http.StatusFound: false, http.StatusBadRequest: false, http.StatusNotFound: false,
		http.StatusRequestTimeout: true, http.StatusTooManyRequests: true, http.StatusInternalServerError: true, http.StatusServiceUnavailable: true,
	}
	receivers := make(map[int]*receiver, statusOptions)
	configs := make([]webhooks.TargetConfig, none, statusOptions)
	names := make([]string, none, statusOptions)
	for status := range retried {
		recv := newReceiver(t, ip, func(_ int, w http.ResponseWriter) int {
			if status == http.StatusFound {
				w.Header().Set("Location", "/elsewhere")
			}
			return status
		})
		receivers[status] = recv
		name := "s" + strconv.Itoa(status)
		names = append(names, name)
		configs = append(configs, target(name, recv.url+hookPath, approval))
	}
	e.writeRaw(t, encode(t, configs))
	e.run(t)
	e.waitGroups(t, names...)
	e.publish(t, approval)
	for status, retry := range retried {
		name, recv := "s"+strconv.Itoa(status), receivers[status]
		want := int64(none)
		if retry {
			want = one
		}
		waitUntil(t, name+" delivery", func() bool { return recv.count() == one })
		waitUntil(t, name+" outcome", func() bool { return e.pending(t, name) == want })
	}
	time.Sleep(quietFor)
	if redirected := receivers[http.StatusFound]; redirected.count() != one || redirected.last().path != hookPath {
		t.Error("a redirect was followed")
	}
}

func TestRetryScheduleAndGiveUp(t *testing.T) {
	e := newEnv(t)
	now := time.Unix(startUnix, none).UTC()
	e.mr.SetTime(now)
	recv := e.serve(t, always(http.StatusInternalServerError))
	e.publish(t, approval)
	waitUntil(t, firstAttempt, func() bool { return recv.count() == one })

	for attempt := two; attempt <= maxAttempts; attempt++ {
		delay := retryDelays[attempt-two]
		e.mr.SetTime(now.Add(delay * earlyPercent / percent))
		time.Sleep(quietFor)
		if recv.count() != attempt-one {
			t.Fatalf("attempt %d came before its delay", attempt)
		}
		now = now.Add(delay * latePercent / percent)
		e.mr.SetTime(now)
		waitUntil(t, fmt.Sprintf("attempt %d", attempt), func() bool { return recv.count() == attempt })
	}
	waitUntil(t, "gave_up acks the entry", func() bool { return e.pending(t, opsName) == none })
	e.mr.SetTime(now.Add(afterLastRetry))
	time.Sleep(quietFor)
	if recv.count() != maxAttempts {
		t.Errorf("%d attempts, want %d", recv.count(), maxAttempts)
	}
}

func TestRetryAfterPushesLater(t *testing.T) {
	values := map[string]func() string{
		"seconds":      func() string { return strconv.Itoa(int(retryAfterWait.Seconds())) },
		"an HTTP date": func() string { return time.Now().Add(retryAfterWait).UTC().Format(http.TimeFormat) },
	}
	for name, value := range values {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			now := time.Unix(startUnix, none).UTC()
			e.mr.SetTime(now)
			recv := e.serve(t, func(n int, w http.ResponseWriter) int {
				if n == one {
					w.Header().Set("Retry-After", value())
					return http.StatusTooManyRequests
				}
				return http.StatusOK
			})
			e.publish(t, approval)
			waitUntil(t, firstAttempt, func() bool { return recv.count() == one })

			e.mr.SetTime(now.Add(pastFirstDelay))
			time.Sleep(quietFor)
			if recv.count() != one {
				t.Fatal("the retry ignored Retry-After")
			}
			e.mr.FastForward(retryAfterWait)
			waitUntil(t, "attempt 2 after Retry-After", func() bool { return recv.count() == two })
			waitUntil(t, theAck, func() bool { return e.pending(t, opsName) == none })
		})
	}
}

func TestTrimmedEntryIsDropped(t *testing.T) {
	e := newEnv(t)
	now := time.Unix(startUnix, none).UTC()
	e.mr.SetTime(now)
	recv := e.serve(t, always(http.StatusBadGateway))
	id := e.publish(t, approval)
	waitUntil(t, firstAttempt, func() bool { return recv.count() == one })
	if err := e.rdb.XDel(context.Background(), notifclient.EventsStream, id).Err(); err != nil {
		t.Fatal(err)
	}
	e.mr.SetTime(now.Add(pastFirstDelay))
	waitUntil(t, "the trimmed entry dropped", func() bool { return e.pending(t, opsName) == none })
	if recv.count() != one {
		t.Errorf("%d attempts, want 1", recv.count())
	}
}

// Every refused address is caught where it is dialed, names that resolve to one included.
func TestAddressGuard(t *testing.T) {
	e := newEnv(t)
	loopback := newReceiver(t, loopbackIP, always(http.StatusOK))
	private := newReceiver(t, privateIP(t), always(http.StatusOK))
	port := loopback.url[strings.LastIndex(loopback.url, portSeparator)+one:]
	refused := []webhooks.TargetConfig{
		target("named-loopback", httpScheme+"localhost:"+port+"/", approval),
		target("loopback", loopback.url+"/", approval),
		target("mapped-loopback", httpScheme+"[::ffff:127.0.0.1]:"+port+"/", approval),
		target("metadata", "http://169.254.169.254/latest/meta-data/", approval),
		{Name: "private", URL: httpsScheme + strings.TrimPrefix(private.url, httpScheme) + "/", Secret: specSecret, Events: []string{approval}},
	}
	allowed := target("allowed", private.url+"/ok", approval)
	names := make([]string, none, len(refused)+one)
	for _, config := range refused {
		names = append(names, config.Name)
	}
	e.writeRaw(t, encode(t, slices.Concat(refused, []webhooks.TargetConfig{allowed})))
	e.run(t)
	e.waitGroups(t, append(names, allowed.Name)...)
	e.publish(t, approval)
	waitUntil(t, "the allowed delivery", func() bool { return private.count() == one })
	for _, name := range names {
		waitUntil(t, name+" refused and acked", func() bool { return e.pending(t, name) == none })
	}
	time.Sleep(quietFor)
	if loopback.count() != none || private.count() != one {
		t.Errorf("loopback got %d, private got %d, want 0 and 1", loopback.count(), private.count())
	}
}

func TestTargetsReload(t *testing.T) {
	e := newEnv(t)
	e.run(t)
	time.Sleep(quietFor)
	if groups := e.groups(); len(groups) != none {
		t.Fatalf("no targets file, yet groups %v", groups)
	}
	e.writeRaw(t, encode(t, []webhooks.TargetConfig{target(nameA, urlA, approval), target(nameB, urlB, approval)}))
	e.waitGroups(t, nameA, nameB)
	e.writeRaw(t, encode(t, []webhooks.TargetConfig{target(nameA, urlA, approval)}))
	e.waitGroups(t, nameA)
	e.writeRaw(t, []byte(`[{"name":"Bad Name"}]`))
	time.Sleep(quietFor)
	if groups := e.groups(); !slices.Contains(groups, groupPrefix+nameA) {
		t.Errorf("a refused file dropped the running target: groups %v", groups)
	}
}

// Redis lost the group (a restart without persistence): the loop makes it again from now.
func TestLostGroupIsRecreated(t *testing.T) {
	e := newEnv(t)
	recv := e.serve(t, always(http.StatusOK))
	if err := e.rdb.XGroupDestroy(context.Background(), notifclient.EventsStream, groupPrefix+opsName).Err(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(quietFor)
	e.waitGroups(t, opsName)
	e.publish(t, approval)
	waitUntil(t, "the delivery after the group came back", func() bool { return recv.count() == one })
}

// A Redis writer can add entries but not sign them: unsigned or altered ones are dropped, never sent, and so is a
// signed one that isn't an event.
func TestUnsignedEntriesAreDropped(t *testing.T) {
	e := newEnv(t)
	recv := e.serve(t, always(http.StatusOK))
	e.publishRaw(t, `{"type":"`+approval+`","timestamp":1,"title":"forged"}`)
	signed, ok := xauthz.SignRedisEntry(notifclient.EventsStream, `{"type":"`+approval+`","timestamp":1,"title":"real"}`)
	garbled, garbledOK := xauthz.SignRedisEntry(notifclient.EventsStream, notJSON)
	if !ok || !garbledOK {
		t.Fatal("no service token to sign with")
	}
	e.publishRaw(t, strings.Replace(signed, "real", "fake", one))
	e.publishRaw(t, garbled)
	waitUntil(t, "all dropped", func() bool { return e.pending(t, opsName) == none })
	e.publish(t, approval)
	waitUntil(t, "the signed delivery", func() bool { return recv.count() == one })
	time.Sleep(quietFor)
	if recv.count() != one {
		t.Errorf("%d deliveries, want only the signed one", recv.count())
	}
}

// A full page of older deliveries, idle but not yet due for their own next attempt, sits ahead of a
// newer one that is due: the scan pages past them, so the newer retry doesn't starve.
func TestRetryScanPagesPastOnePage(t *testing.T) {
	e := newEnv(t)
	now := time.Unix(startUnix, none).UTC()
	e.mr.SetTime(now)
	recv := e.serve(t, always(http.StatusInternalServerError))
	for range pendingPage {
		e.publish(t, approval)
	}
	waitUntil(t, "every first attempt", func() bool { return recv.count() == pendingPage })
	e.mr.SetTime(now.Add(pastFirstDelay))
	waitUntil(t, "every second attempt", func() bool { return recv.count() == pendingPage+pendingPage })
	e.publish(t, approval)
	waitUntil(t, "the newer first attempt", func() bool { return recv.count() == pendingPage+pendingPage+one })
	e.mr.SetTime(now.Add(pastFirstDelay + pastFirstDelay))
	waitUntil(t, "the newer second attempt", func() bool { return recv.count() == pendingPage+pendingPage+two })
}

// Redis down when a target starts: its group is made once Redis answers again, and deliveries go out from then on.
func TestATargetStartedDuringAnOutageRecovers(t *testing.T) {
	e := newEnv(t)
	recv := newReceiver(t, privateIP(t), always(http.StatusOK))
	e.writeRaw(t, encode(t, []webhooks.TargetConfig{target(opsName, recv.url, approval)}))
	e.mr.SetError(noGroupError)
	e.run(t)
	time.Sleep(quietFor)
	e.mr.SetError(dataconstants.EmptyString)
	e.waitGroups(t, opsName)
	e.publish(t, approval)
	waitUntil(t, "the delivery after the outage", func() bool { return recv.count() == one })
}

// A targets file that can't be read starts nothing, as no file does.
func TestATargetsFileThatCantBeReadStartsNothing(t *testing.T) {
	e := newEnv(t)
	e.path = t.TempDir()
	e.run(t)
	time.Sleep(quietFor)
	if groups := e.groups(); len(groups) != none {
		t.Fatalf("an unreadable targets file made groups %v", groups)
	}
}

// An ack Redis refuses leaves the entry pending: delivered again later rather than lost.
func TestARefusedAckKeepsTheEntryPending(t *testing.T) {
	e := newEnv(t)
	recv := e.serve(t, func(int, http.ResponseWriter) int {
		e.mr.SetError(outage)
		return http.StatusOK
	})
	e.publish(t, approval)
	waitUntil(t, firstAttempt, func() bool { return recv.count() == one })
	time.Sleep(quietFor)
	e.mr.SetError(dataconstants.EmptyString)
	if pending := e.pending(t, opsName); pending != one {
		t.Errorf("pending after a refused ack = %d, want %d", pending, one)
	}
}

// A receiver that can't be reached is retried, not given up: the entry stays pending.
func TestAnUnreachableReceiverIsRetried(t *testing.T) {
	e := newEnv(t)
	listener, err := net.Listen("tcp", net.JoinHostPort(privateIP(t), "0"))
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	e.writeRaw(t, encode(t, []webhooks.TargetConfig{target(opsName, httpScheme+address+hookPath, approval)}))
	e.run(t)
	e.waitGroups(t, opsName)
	e.publish(t, approval)
	waitUntil(t, "the refused attempt", func() bool { return e.pending(t, opsName) == one })
	time.Sleep(quietFor)
	if pending := e.pending(t, opsName); pending != one {
		t.Errorf("pending after a refused connection = %d, want %d", pending, one)
	}
}

// An entry Redis counts as delivered as often as allowed, its last answer never recorded (a restart), is given up.
func TestAnEntryPastItsLastAttemptIsGivenUp(t *testing.T) {
	e := newEnv(t)
	now := time.Unix(startUnix, none).UTC()
	e.mr.SetTime(now)
	recv := e.serve(t, always(http.StatusInternalServerError))
	id := e.publish(t, approval)
	waitUntil(t, firstAttempt, func() bool { return recv.count() == one })
	claim := []any{"XCLAIM", notifclient.EventsStream, groupPrefix + opsName, consumer, noIdle, id, retryCount, maxAttempts}
	if err := e.rdb.Do(context.Background(), claim...).Err(); err != nil {
		t.Fatal(err)
	}
	e.mr.SetTime(now.Add(afterLastRetry))
	waitUntil(t, "the entry given up", func() bool { return e.pending(t, opsName) == none })
	if recv.count() != one {
		t.Errorf("%d attempts, want 1", recv.count())
	}
}
