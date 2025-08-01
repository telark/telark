package core

type RedisManagerInterface interface {
	GetClient() (*RedisClient, error)
	IsConnected() bool
	Close() error
	InitRedisClient() (*RedisClient, error)
	Reconnect() error
	GetConnectionStatus() (bool, error)
}
