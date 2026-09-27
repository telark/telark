package insights

import (
	"github.com/telark/data/resources/application"
	xwareredis "github.com/telark/x-ware/redis/stream"
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
