package informers

import (
	"context"

	applicationmodel "github.com/telark/data/resources/application"
	"github.com/redis/go-redis/v9"
)

type Config struct {
	RDB          *redis.Client
	LeaderFn     func(context.Context) bool
	ReplicaID    string
	DiscoverApps func(context.Context, *redis.Client) ([]applicationmodel.Application, error)
	RunPrewarmNS func(context.Context, *redis.Client, string) error
}
