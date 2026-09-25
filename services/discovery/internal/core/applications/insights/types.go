package insights

import xwareredis "github.com/telark/x-ware/redis/stream"

type Logger interface {
	Warn(msg string)
}

type Publisher struct {
	stream *xwareredis.StreamClient
	lg     Logger
}
