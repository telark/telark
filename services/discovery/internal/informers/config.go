package informers

import (
	"context"

	"github.com/redis/go-redis/v9"
	applicationmodel "github.com/telark/data/resources/application"
)

type Config struct {
	RDB          *redis.Client
	LeaderFn     func(context.Context) bool
	ReplicaID    string
	DiscoverApps func(context.Context, *redis.Client) ([]applicationmodel.Application, error)
	RunPrewarmNS func(context.Context, *redis.Client, string) error
}
