package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/A-small-character/cmdgen-platform/pkg/config"
	"github.com/redis/go-redis/v9"
)

// Cache 缓存接口
type Cache interface {
	Get(ctx context.Context, key string, dest interface{}) error
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(cfg *config.RedisConfig) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("连接Redis失败: %w", err)
	}

	return &RedisCache{client: client}, nil
}

func (c *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
	data, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return fmt.Errorf("cache miss: %s", key)
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, ttl).Err()
}

func (c *RedisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.client.Exists(ctx, key).Result()
	return n > 0, err
}

// GenerateCacheKey 生成缓存Key
func GenerateCacheKey(category, input string) string {
	return fmt.Sprintf("cmdgen:result:%s:%x", category, hashString(input))
}

func hashString(s string) uint64 {
	var h uint64 = 14695981039346656037
	for _, c := range []byte(s) {
		h ^= uint64(c)
		h *= 1099511628211
	}
	return h
}
