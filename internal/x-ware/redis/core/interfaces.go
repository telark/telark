package core

import "context"

type RedisManagerInterface interface {
	GetClient() (*RedisClient, error)
	IsConnected() bool
	Close() error
	InitRedisClient() (*RedisClient, error)
	Reconnect() error
	GetConnectionStatus() (bool, error)
	GetPoolStats() (*PoolStats, error)
	HealthCheck(ctx context.Context) error
	GetConnectionInfo() (*ConnectionInfo, error)
}

type PoolStats struct {
	TotalConns uint32
	IdleConns  uint32
	StaleConns uint32
	WaitCount  uint32
}

type ConnectionInfo struct {
	ClientName              string
	ClientID                int64
	ConnectedAt             int64
	LastCommandAt           int64
	Database                int
	Flags                   string
	Subscriptions           int
	PatternSubscriptions    int
	Channels                int
	BlockedCommands         int
	BlockedCommandsDuration int64
}
