package session

import "github.com/redis/go-redis/v9"

type RedisSession struct {
	redis *redis.Client
}

func NewRedisSession(redisClient *redis.Client) *RedisSession {
	return &RedisSession{redis: redisClient}
}
