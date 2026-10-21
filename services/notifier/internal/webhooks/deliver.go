package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	notifclient "github.com/telark/telark/internal/rest/clients/notifications"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/notifier/internal/constants"
)

var (
	errUnsignedEntry = errors.New(constants.ErrWebhookUnsigned)
	errBadEntry      = errors.New(constants.ErrWebhookBadEntry)
)

// The first retry waits the shortest jittered delay; nothing idle for less is ever due.
var minRetryIdle = constants.WebhookRetryDelays[constants.DefaultInitValue] *
	(constants.WebhookPercent - constants.WebhookJitterPercent) / constants.WebhookPercent

// New entries are read without blocking, so no target holds a pooled Redis connection while idle.
func (d *deliverer) run(ctx context.Context) {
	poll := time.NewTicker(d.poll)
	defer poll.Stop()
	scan := time.NewTicker(d.retryScan)
	defer scan.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-poll.C:
			d.readNew(ctx)
		case <-scan.C:
			d.retryDue(ctx)
		}
	}
}

func (d *deliverer) readNew(ctx context.Context) {
	for ctx.Err() == nil {
		messages, err := d.stream.Consume(
			ctx, notifclient.EventsStream, d.group, d.consumer, constants.WebhookReadBatch, constants.WebhookNoBlock,
		)
		if err != nil {
			d.streamFailed(ctx, err)
			return
		}
		d.failing = false
		for _, msg := range messages {
			d.attempt(ctx, msg, constants.DefaultAdd)
		}
		if len(messages) < constants.WebhookReadBatch {
			return
		}
	}
}

// An entry stays pending until it is delivered or given up, so a retry survives a restart and
// moves to a live replica: XCLAIM's minimum idle lets only one replica take it.
// Paged from the oldest on, so a dead receiver's backlog never starves the newer retries.
func (d *deliverer) retryDue(ctx context.Context) {
	for start, more := constants.WebhookStreamStart, true; more && ctx.Err() == nil; {
		pending, err := d.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
			Stream: notifclient.EventsStream,
			Group:  d.group,
			Idle:   minRetryIdle,
			Start:  start,
			End:    constants.WebhookStreamEnd,
			Count:  constants.WebhookPendingBatch,
		}).Result()
		if err != nil {
			d.streamFailed(ctx, err)
			return
		}
		for _, entry := range pending {
			d.retry(ctx, entry)
		}
		more = len(pending) == constants.WebhookPendingBatch
		if more {
			start, more = nextID(pending[len(pending)-constants.DefaultAdd].ID)
		}
	}
}

// The smallest id after this one, so a page starts past the last entry of the previous one.
func nextID(id string) (string, bool) {
	millis, sequence, found := strings.Cut(id, constants.WebhookIDSeparator)
	next, err := strconv.ParseUint(sequence, constants.WebhookDecimal, constants.WebhookUint64Bits)
	if !found || err != nil || next == math.MaxUint64 {
		return constants.EmptyString, false
	}
	return millis + constants.WebhookIDSeparator + strconv.FormatUint(next+constants.DefaultAdd, constants.WebhookDecimal), true
}

func (d *deliverer) retry(ctx context.Context, entry redis.XPendingExt) {
	// Redis is not trusted: a delivery count it reports below one must not index the delays.
	deliveries := max(entry.RetryCount, constants.DefaultAdd)
	attempt := int(deliveries) + constants.DefaultAdd
	if attempt > constants.WebhookMaxAttempts {
		// Its last attempt ended without an answer recorded, a restart for example.
		d.ack(ctx, entry.ID)
		gaveUp := attemptResult{outcome: constants.WebhookOutcomeGaveUp}
		d.logAttempt(entry.ID, constants.EmptyString, constants.WebhookMaxAttempts, gaveUp, constants.DefaultInitValue)
		return
	}
	delay := retryDelay(entry.ID, deliveries)
	if entry.Idle < delay || d.pushedBack(ctx, entry.ID) {
		return
	}
	claimed, err := d.rdb.XClaim(ctx, &redis.XClaimArgs{
		Stream:   notifclient.EventsStream,
		Group:    d.group,
		Consumer: d.consumer,
		MinIdle:  delay,
		Messages: []string{entry.ID},
	}).Result()
	switch {
	case err != nil:
		d.streamFailed(ctx, err)
	case len(claimed) == constants.DefaultInitValue:
		d.dropIfTrimmed(ctx, entry.ID)
	default:
		d.attempt(ctx, claimed[constants.DefaultInitValue], attempt)
	}
}

// Jittered ±10 % per entry and attempt, the same on every scan, so a rescan never re-rolls it.
func retryDelay(id string, deliveries int64) time.Duration {
	base := constants.WebhookRetryDelays[deliveries-constants.DefaultAdd]
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(id + strconv.FormatInt(deliveries, constants.WebhookDecimal)))
	spread := int64(hash.Sum32()%(constants.WebhookJitterPercent+constants.WebhookJitterPercent+constants.DefaultAdd)) -
		constants.WebhookJitterPercent
	return base + base*time.Duration(spread)/constants.WebhookPercent
}

// A claim that returns nothing means another replica took the entry, or the stream trimmed it
// while it waited, which caps a dead receiver's backlog.
func (d *deliverer) dropIfTrimmed(ctx context.Context, id string) {
	entries, err := d.rdb.XRange(ctx, notifclient.EventsStream, id, id).Result()
	if err != nil || len(entries) > constants.DefaultInitValue {
		return
	}
	d.ack(ctx, id)
	d.log.Error(fmt.Sprintf(constants.LogWebhookDropped, d.target.Config.Name, id, constants.ErrWebhookTrimmed))
}

// An entry that doesn't verify, or a type the target doesn't list (any longer, after an edit), is
// acknowledged without a delivery.
func (d *deliverer) attempt(ctx context.Context, msg redis.XMessage, attempt int) {
	event, err := readEvent(msg)
	var body []byte
	if err == nil {
		body, err = buildPayload(msg.ID, event)
	}
	if err != nil || !slices.Contains(d.target.Config.Events, event.Type) {
		d.ack(ctx, msg.ID)
		if err != nil {
			d.log.Error(fmt.Sprintf(constants.LogWebhookDropped, d.target.Config.Name, msg.ID, err))
		}
		return
	}
	eventType := event.Type
	started := time.Now()
	result := d.send(ctx, msg.ID, body)
	if result.outcome == constants.WebhookOutcomeRetry && attempt >= constants.WebhookMaxAttempts {
		result.outcome = constants.WebhookOutcomeGaveUp
	}
	if result.outcome != constants.WebhookOutcomeRetry {
		d.ack(ctx, msg.ID)
	} else if result.retryAfter > constants.DefaultInitValue {
		d.rdb.Set(ctx, d.pushbackKey(msg.ID), constants.WebhookPushbackValue, result.retryAfter)
	}
	d.logAttempt(msg.ID, eventType, attempt, result, time.Since(started))
}

func (d *deliverer) send(ctx context.Context, id string, body []byte) attemptResult {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, d.target.Config.URL, bytes.NewReader(body))
	if err != nil {
		return attemptResult{outcome: constants.WebhookOutcomeFailed, err: errBadURL}
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), constants.WebhookDecimal)
	request.Header.Set(constants.HeaderContentType, constants.ContentTypeJSON)
	request.Header.Set(constants.HeaderUserAgent, constants.WebhookUserAgent)
	request.Header.Set(constants.HeaderWebhookID, id)
	request.Header.Set(constants.HeaderWebhookTimestamp, timestamp)
	request.Header.Set(constants.HeaderWebhookSignature, sign(d.target.keys, id, timestamp, body))
	response, err := d.client.Do(request)
	if err != nil {
		if errors.Is(err, errAddressNotAllowed) {
			return attemptResult{outcome: constants.WebhookOutcomeFailed, err: err}
		}
		return attemptResult{outcome: constants.WebhookOutcomeRetry, err: err}
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, constants.WebhookMaxResponseBytes))
	result := attemptResult{outcome: classify(response.StatusCode), status: response.StatusCode}
	if result.outcome == constants.WebhookOutcomeRetry {
		result.retryAfter = retryAfter(response.Header.Get(constants.HeaderRetryAfter))
	}
	return result
}

// Standard Webhooks: v1 HMAC-SHA256 over "id.timestamp.body", one signature per key.
func sign(keys [][]byte, id, timestamp string, body []byte) string {
	signatures := make([]string, constants.DefaultInitValue, len(keys))
	for _, key := range keys {
		mac := hmac.New(sha256.New, key)
		_, _ = mac.Write([]byte(id + constants.WebhookSignedSeparator + timestamp + constants.WebhookSignedSeparator))
		_, _ = mac.Write(body)
		signatures = append(signatures, constants.WebhookSignaturePrefix+base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	}
	return strings.Join(signatures, constants.WebhookSignatureJoin)
}

// Redirects are never followed, so a 3xx fails like any other 4xx but 408 and 429.
func classify(status int) string {
	switch {
	case status >= http.StatusOK && status < http.StatusMultipleChoices:
		return constants.WebhookOutcomeDelivered
	case status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= http.StatusInternalServerError:
		return constants.WebhookOutcomeRetry
	default:
		return constants.WebhookOutcomeFailed
	}
}

// It can only push the next attempt later, and by 4 h at most.
func retryAfter(value string) time.Duration {
	var wait time.Duration
	if seconds, err := strconv.Atoi(value); err == nil {
		wait = time.Duration(seconds) * time.Second
	} else if at, err := http.ParseTime(value); err == nil {
		wait = time.Until(at)
	}
	return min(max(wait, constants.DefaultInitValue), constants.WebhookMaxRetryAfter)
}

func (d *deliverer) pushedBack(ctx context.Context, id string) bool {
	return d.rdb.Exists(ctx, d.pushbackKey(id)).Val() > constants.DefaultInitValue
}

func (d *deliverer) pushbackKey(id string) string {
	return constants.WebhookPushbackPrefix + d.target.Config.Name + constants.WebhookNameKeySeparator + id
}

func (d *deliverer) ack(ctx context.Context, id string) {
	if err := d.stream.Ack(ctx, notifclient.EventsStream, d.group, id); err != nil {
		d.streamFailed(ctx, err)
	}
}

// Logged once per outage. A group lost with Redis's data is made again from now on.
func (d *deliverer) streamFailed(ctx context.Context, err error) {
	if ctx.Err() != nil {
		return
	}
	if !d.failing {
		d.log.Warn(fmt.Sprintf(constants.ErrWebhookStream, d.target.Config.Name, err))
	}
	d.failing = true
	if strings.Contains(err.Error(), constants.WebhookNoGroupError) {
		if err := d.stream.EnsureConsumerGroupFromNow(ctx, notifclient.EventsStream, d.group); err != nil {
			d.log.Warn(fmt.Sprintf(constants.ErrWebhookGroup, d.target.Config.Name, err))
		}
	}
}

// One line per attempt: never the URL or a secret, and the error cut short.
func (d *deliverer) logAttempt(id, eventType string, attempt int, result attemptResult, took time.Duration) {
	status := constants.WebhookStatusNone
	if result.status != constants.DefaultInitValue {
		status = strconv.Itoa(result.status)
	}
	line := fmt.Sprintf(constants.LogWebhookAttempt, d.target.Config.Name, id, eventType, attempt, constants.WebhookMaxAttempts,
		status, took.Round(time.Millisecond), result.outcome, errorText(result.err))
	switch result.outcome {
	case constants.WebhookOutcomeDelivered:
		d.log.Info(line)
	case constants.WebhookOutcomeRetry:
		d.log.Warn(line)
	default:
		d.log.Error(line)
	}
}

// http.Client errors quote the whole URL, which can carry a credential (Slack-style hooks).
func errorText(err error) string {
	if err == nil {
		return constants.EmptyString
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}
	text := err.Error()
	if len(text) > constants.WebhookMaxErrorText {
		text = text[:constants.WebhookMaxErrorText]
	}
	return text
}

// Redis is untrusted: only an entry signed with the service token for this stream is an event.
func readEvent(msg redis.XMessage) (notifclient.QueuedEvent, error) {
	var event notifclient.QueuedEvent
	raw, isString := msg.Values[notifclient.EventField].(string)
	payload, ok := xauthz.VerifyRedisEntry(notifclient.EventsStream, raw)
	if !isString || !ok {
		return event, errUnsignedEntry
	}
	if json.Unmarshal([]byte(payload), &event) != nil {
		return event, errBadEntry
	}
	return event, nil
}

func buildPayload(id string, event notifclient.QueuedEvent) ([]byte, error) {
	return json.Marshal(payload{
		Version:   constants.WebhookPayloadVersion,
		ID:        id,
		Type:      event.Type,
		Timestamp: time.UnixMilli(event.Timestamp).UTC().Format(constants.WebhookTimestampLayout),
		Data: payloadData{
			Severity: event.Severity,
			Title:    event.Title,
			Message:  event.Message,
			Metadata: event.Metadata,
		},
	})
}
