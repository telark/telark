package insights

import (
	"github.com/telark/telark/internal/data/resources/application"
	xwareredis "github.com/telark/telark/internal/x-ware/redis/stream"
)

type Logger interface {
	Warn(msg string)
}

type StoredApplicationFn func(name string) (*application.Application, error)

type Publisher struct {
	stream *xwareredis.StreamClient
	lg     Logger
	stored StoredApplicationFn
}
