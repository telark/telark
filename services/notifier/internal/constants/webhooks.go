package constants

import "time"

const (
	PrefixWebhooks            = "Webhooks: "
	EnvWebhookTargetsFile     = "WEBHOOK_TARGETS_FILE"
	DefaultWebhookTargetsFile = "/etc/telark/webhooks/targets.json"
	// The kubelet refreshes a mounted Secret within about a minute, so this adds at most half of that.
	WebhookReloadInterval    = 30 * time.Second
	WebhookPollInterval      = time.Second
	WebhookRetryScanInterval = 10 * time.Second
	WebhookGroupPrefix       = "webhook:"
	WebhookPushbackPrefix    = "webhook:retry-after:"
	WebhookPushbackValue     = 1
	// One at a time: an entry read but not yet sent would idle long enough for another replica to claim it.
	WebhookReadBatch        = 1
	WebhookPendingBatch     = 100
	WebhookNoBlock          = -1
	WebhookMaxAttempts      = 8
	WebhookAttemptTimeout   = 10 * time.Second
	WebhookDialTimeout      = 5 * time.Second
	WebhookMaxRetryAfter    = 4 * time.Hour
	WebhookJitterPercent    = 10
	WebhookPercent          = 100
	WebhookMaxResponseBytes = 64 << 10
	WebhookMaxErrorText     = 200
	// The base64 characters of a secret, 24 bytes at least; values.schema.json holds the same number.
	WebhookMinSecretChars    = 32
	WebhookBase64Padding     = "="
	WebhookIdleConnTimeout   = 90 * time.Second
	WebhookSecretPrefix      = "whsec_"
	WebhookSignaturePrefix   = "v1,"
	WebhookSignatureJoin     = " "
	WebhookSignedSeparator   = "."
	WebhookPayloadVersion    = 1
	WebhookUserAgent         = "telark-webhooks/1"
	WebhookNamePattern       = `^[a-z0-9-]+$`
	WebhookTimestampLayout   = "2006-01-02T15:04:05.000Z07:00"
	WebhookSchemeHTTPS       = "https"
	WebhookSchemeHTTP        = "http"
	WebhookHostLocal         = "localhost"
	WebhookHostDot           = "."
	WebhookHostSuffixSvc     = ".svc"
	WebhookHostSuffixCluster = ".cluster.local"
	WebhookDecimal           = 10
	WebhookUint64Bits        = 64
	WebhookIDSeparator       = "-"
	WebhookStatusNone        = "-"
	WebhookStreamStart       = "-"
	WebhookStreamEnd         = "+"
	WebhookNoGroupError      = "NOGROUP"
	WebhookNoSuchKeyError    = "no such key"
	WebhookNoTargets         = "[]"
	WebhookNameJoin          = ", "
	WebhookNameKeySeparator  = ":"
	WebhookOutcomeDelivered  = "delivered"
	WebhookOutcomeRetry      = "retry"
	WebhookOutcomeFailed     = "failed"
	WebhookOutcomeGaveUp     = "gave_up"
	HeaderWebhookID          = "webhook-id"
	HeaderWebhookTimestamp   = "webhook-timestamp"
	HeaderWebhookSignature   = "webhook-signature"
	HeaderContentType        = "Content-Type"
	HeaderUserAgent          = "User-Agent"
	HeaderRetryAfter         = "Retry-After"
	ContentTypeJSON          = "application/json"
	// Private and cluster ranges a target reaches only with allowPrivate; loopback, link-local,
	// multicast and the unspecified range are refused always.
	CIDRCarrierGradeNAT = "100.64.0.0/10"
	CIDRThisNetwork     = "0.0.0.0/8"
	IPBroadcast         = "255.255.255.255"
)

// The waits before attempts 2 to 8, measured from the previous attempt: about 8 h in all.
var WebhookRetryDelays = []time.Duration{
	30 * time.Second, 2 * time.Minute, 10 * time.Minute, 30 * time.Minute, time.Hour, 2 * time.Hour, 4 * time.Hour,
}

const (
	InfoWebhookTargetsLoaded = "loaded %d webhook target(s) [%s]"
	LogWebhookAttempt        = "webhook target=%s event=%s type=%s attempt=%d/%d status=%s duration=%s outcome=%s error=%q"
	LogWebhookDropped        = "webhook target=%s event=%s outcome=dropped reason=%q"
	ErrWebhookTargetsRead    = "failed to read the webhook targets file: %v"
	ErrWebhookTargetsRefused = "webhook targets refused, the previous ones stay in place: %v"
	ErrWebhookGroup          = "webhook target %s: consumer group: %v"
	ErrWebhookStream         = "webhook target %s: event stream: %v"
	ErrWebhookTargetsJSON    = "targets.json is not a list of targets: %v"
	ErrWebhookTarget         = "target %d (%q): %w"
	ErrWebhookDuplicateName  = "the name is used twice"
	ErrWebhookBadName        = "name must match [a-z0-9-]+"
	ErrWebhookNoEvents       = "events is empty"
	ErrWebhookBadEvent       = "event %q can't be sent to a webhook"
	ErrWebhookBadURL         = "url doesn't parse"
	ErrWebhookURLScheme      = "url must use https (http needs allowPrivate)"
	ErrWebhookURLUserInfo    = "url must not carry a user or password"
	ErrWebhookURLPrivateHost = "url names a local or cluster host, which needs allowPrivate"
	ErrWebhookBadSecret      = "secret must be whsec_ and at least 32 base64 characters"
	ErrWebhookAddress        = "address not allowed"
	ErrWebhookBadEntry       = "malformed event entry"
	ErrWebhookUnsigned       = "unsigned or altered event entry"
	ErrWebhookTrimmed        = "trimmed from the stream before delivery"
)
