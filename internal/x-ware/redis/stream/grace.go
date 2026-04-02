package stream

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type ScaleGrace struct {
	DetectedAt       time.Time `json:"detectedAt"`
	ExpectedReplicas int       `json:"expectedReplicas"`
}

type GraceClient struct {
	redis *redis.Client
}

func NewGraceClient(r *redis.Client) *GraceClient {
	return &GraceClient{redis: r}
}

func (c *GraceClient) SetScaleGrace(ctx context.Context, appName string, expectedReplicas int, ttl time.Duration) error {
	data, err := json.Marshal(ScaleGrace{
		DetectedAt:       time.Now().UTC(),
		ExpectedReplicas: expectedReplicas,
	})
	if err != nil {
		return err
	}
	return c.redis.Set(ctx, "grace:scale:"+appName, data, ttl).Err()
}

func (c *GraceClient) GetScaleGrace(ctx context.Context, appName string) (*ScaleGrace, error) {
	data, err := c.redis.Get(ctx, "grace:scale:"+appName).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	var grace ScaleGrace
	if err := json.Unmarshal([]byte(data), &grace); err != nil {
		return nil, err
	}
	return &grace, nil
}
