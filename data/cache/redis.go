package cache

import "github.com/redis/go-redis/v9"

type RedisCache struct {
	redis *redis.Client
}

func NewRedisCache(redisClient *redis.Client) *RedisCache {
	return &RedisCache{redis: redisClient}
}