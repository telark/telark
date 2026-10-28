package webhooks

import (
	"context"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/internal/data/logger"
	xstream "github.com/telark/telark/internal/x-ware/redis/stream"
)

// One entry of targets.json, the list the chart renders from app.webhooks.targets.
type TargetConfig struct {
	Name           string   `json:"name"`
	URL            string   `json:"url"`
	Secret         string   `json:"secret"`
	PreviousSecret string   `json:"previousSecret,omitempty"`
	Events         []string `json:"events"`
	AllowPrivate   bool     `json:"allowPrivate,omitempty"`
}

// A validated target and the signing keys decoded from its secrets.
type Target struct {
	Config TargetConfig
	keys   [][]byte
}

// The intervals are arguments, so tests run the real loops on short ones.
type Service struct {
	rdb       *redis.Client
	stream    *xstream.StreamClient
	consumer  string
	path      string
	reload    time.Duration
	poll      time.Duration
	retryScan time.Duration
	log       *logger.CustomLogger
	loops     map[string]*loop
}

type loop struct {
	target Target
	client *http.Client
	cancel context.CancelFunc
	done   chan struct{}
}

type deliverer struct {
	*Service
	target  Target
	client  *http.Client
	group   string
	failing bool
}

type attemptResult struct {
	outcome    string
	status     int
	retryAfter time.Duration
	err        error
}

type payload struct {
	Version   int         `json:"version"`
	ID        string      `json:"id"`
	Type      string      `json:"type"`
	Timestamp string      `json:"timestamp"`
	Data      payloadData `json:"data"`
}

type payloadData struct {
	Severity string         `json:"severity"`
	Title    string         `json:"title"`
	Message  string         `json:"message"`
	Metadata map[string]any `json:"metadata,omitempty"`
}
