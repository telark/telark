package grace

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	rediscore "github.com/plsyro/x-ware/redis/core"
	redisv9 "github.com/redis/go-redis/v9"
)

const (
	keyPrefixScaleGrace = "grace:scale:"
)

type ScaleGrace struct {
	DetectedAt       time.Time `json:"detectedAt"`
	ExpectedReplicas int       `json:"expectedReplicas"`
}

type Client struct {
	rm rediscore.RedisManagerInterface
}

func NewClient(rm rediscore.RedisManagerInterface) *Client {
	if rm == nil {
		rm = rediscore.NewRedisManager()
	}
	return &Client{rm: rm}
}

func (c *Client) SetScaleGrace(
	ctx context.Context,
	appName string,
	expectedReplicas int,
	ttl time.Duration,
) error {
	if c == nil || c.rm == nil {
		return fmt.Errorf("grace redis manager is nil")
	}
	rc, err := c.rm.GetClient()
	if err != nil {
		return err
	}
	val := ScaleGrace{DetectedAt: time.Now().UTC(), ExpectedReplicas: expectedReplicas}
	b, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return rc.Client.Set(ctx, keyPrefixScaleGrace+appName, string(b), ttl).Err()
}

func (c *Client) GetScaleGrace(ctx context.Context, appName string) (*ScaleGrace, error) {
	if c == nil || c.rm == nil {
		return nil, fmt.Errorf("grace redis manager is nil")
	}
	rc, err := c.rm.GetClient()
	if err != nil {
		return nil, err
	}
	s, err := rc.Client.Get(ctx, keyPrefixScaleGrace+appName).Result()
	if err != nil {
		if err == redisv9.Nil {
			return nil, nil
		}
		return nil, err
	}
	var out ScaleGrace
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
