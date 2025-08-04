package ratelimiting

import (
	"sync"
	"time"
)

type RateLimiter struct {
	interval time.Duration
	lastCall time.Time
	mu       sync.Mutex
}

func NewRateLimiter(interval time.Duration) *RateLimiter {
	return &RateLimiter{
		interval: interval,
		lastCall: time.Time{},
	}
}

func (rl *RateLimiter) Wait() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	timeSinceLastCall := now.Sub(rl.lastCall)

	if timeSinceLastCall < rl.interval {
		sleepTime := rl.interval - timeSinceLastCall
		time.Sleep(sleepTime)
	}

	rl.lastCall = time.Now()
}

func (rl *RateLimiter) Try() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	timeSinceLastCall := now.Sub(rl.lastCall)

	if timeSinceLastCall >= rl.interval {
		rl.lastCall = now
		return true
	}

	return false
}
