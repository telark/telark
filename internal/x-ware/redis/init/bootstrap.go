package init

import "sync/atomic"

type BootstrapState struct {
	ready atomic.Bool
}

func (b *BootstrapState) SetReady() {
	if b == nil {
		return
	}
	b.ready.Store(true)
}

func (b *BootstrapState) IsReady() bool {
	if b == nil {
		return false
	}
	return b.ready.Load()
}

func (b *BootstrapState) Reset() {
	if b == nil {
		return
	}
	b.ready.Store(false)
}
