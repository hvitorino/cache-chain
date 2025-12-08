package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"cache-chain/pkg/cache"

	"github.com/redis/rueidis"
)

type RedisCache struct {
	client rueidis.Client
	name   string
	config RedisCacheConfig
}

type RedisCacheConfig struct {
	Name string
	// Addr is the Redis server address for single node/sentinel mode.
	// For cluster mode, use ClusterAddrs instead.
	// Examples: "localhost:6379", "redis.example.com:6379"
	Addr string
	// ClusterAddrs is a list of Redis cluster node addresses.
	// If set, cluster mode is enabled automatically.
	// Example: []string{"node1:6379", "node2:6379", "node3:6379"}
	ClusterAddrs []string
	Username     string
	Password     string
	// DB is the Redis database number (0-15).
	// Note: In cluster mode, only DB 0 is supported.
	DB               int
	KeyPrefix        string
	MaxRetries       int
	DialTimeout      time.Duration
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	PoolSize         int
	MinIdleConns     int
	EnablePipelining bool
	// Sentinel configuration for high availability
	SentinelMasterSet string
	// SentinelAddrs is a list of Redis Sentinel addresses.
	// If set, sentinel mode is enabled.
	SentinelAddrs    []string
	SentinelUsername string
	SentinelPassword string
}

func DefaultRedisCacheConfig() RedisCacheConfig {
	return RedisCacheConfig{
		Name:             "Redis",
		Addr:             "localhost:6379",
		Username:         "",
		Password:         "",
		DB:               0,
		KeyPrefix:        "cache:",
		MaxRetries:       3,
		DialTimeout:      5 * time.Second,
		ReadTimeout:      3 * time.Second,
		WriteTimeout:     3 * time.Second,
		PoolSize:         10,
		MinIdleConns:     2,
		EnablePipelining: true,
	}
}

// ClusterCacheConfig returns a configuration for Redis Cluster mode.
// clusterAddrs should contain multiple Redis cluster node addresses.
func ClusterCacheConfig(name string, clusterAddrs []string, password string) RedisCacheConfig {
	config := DefaultRedisCacheConfig()
	config.Name = name
	config.ClusterAddrs = clusterAddrs
	config.Password = password
	config.Addr = "" // Clear single node address
	config.DB = 0    // Cluster only supports DB 0
	return config
}

// SentinelCacheConfig returns a configuration for Redis Sentinel mode.
// sentinelAddrs should contain Redis Sentinel addresses.
// masterSet is the name of the master set to connect to.
func SentinelCacheConfig(name string, sentinelAddrs []string, masterSet, password string) RedisCacheConfig {
	config := DefaultRedisCacheConfig()
	config.Name = name
	config.SentinelAddrs = sentinelAddrs
	config.SentinelMasterSet = masterSet
	config.Password = password
	config.Addr = "" // Clear single node address
	return config
}

func NewRedisCache(config RedisCacheConfig) (*RedisCache, error) {
	if config.Name == "" {
		config.Name = "Redis"
	}

	// Determine addresses based on configuration
	var initAddress []string
	if len(config.ClusterAddrs) > 0 {
		// Cluster mode: use multiple addresses
		initAddress = config.ClusterAddrs
	} else if len(config.SentinelAddrs) > 0 {
		// Sentinel mode: use sentinel addresses
		initAddress = config.SentinelAddrs
	} else if config.Addr != "" {
		// Single node mode
		initAddress = []string{config.Addr}
	} else {
		return nil, fmt.Errorf("redis: no addresses configured (set Addr, ClusterAddrs, or SentinelAddrs)")
	}

	clientOpts := rueidis.ClientOption{
		InitAddress:      initAddress,
		Username:         config.Username,
		Password:         config.Password,
		SelectDB:         config.DB,
		ConnWriteTimeout: config.WriteTimeout,
		MaxFlushDelay:    100 * time.Microsecond,
	}

	// Configure Sentinel if enabled
	if len(config.SentinelAddrs) > 0 {
		clientOpts.Sentinel = rueidis.SentinelOption{
			MasterSet: config.SentinelMasterSet,
			Username:  config.SentinelUsername,
			Password:  config.SentinelPassword,
		}
	}

	client, err := rueidis.NewClient(clientOpts)
	if err != nil {
		return nil, fmt.Errorf("redis: failed to create client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.DialTimeout)
	defer cancel()

	if err := client.Do(ctx, client.B().Ping().Build()).Error(); err != nil {
		client.Close()
		return nil, fmt.Errorf("redis: failed to ping server: %w", err)
	}

	return &RedisCache{
		client: client,
		name:   config.Name,
		config: config,
	}, nil
}

func (r *RedisCache) Get(ctx context.Context, key string) (interface{}, error) {
	fullKey := r.config.KeyPrefix + key

	cmd := r.client.B().Get().Key(fullKey).Build()
	resp := r.client.Do(ctx, cmd)

	if err := resp.Error(); err != nil {
		if rueidis.IsRedisNil(err) {
			return nil, cache.ErrCacheMiss
		}
		return nil, fmt.Errorf("redis get: %w", err)
	}

	data, err := resp.AsBytes()
	if err != nil {
		return nil, fmt.Errorf("redis get: failed to read response: %w", err)
	}

	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("redis get: failed to unmarshal: %w", err)
	}

	return value, nil
}

func (r *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	fullKey := r.config.KeyPrefix + key

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("redis set: failed to marshal: %w", err)
	}

	cmd := r.client.B().Set().Key(fullKey).Value(string(data)).Ex(ttl).Build()
	if err := r.client.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}

	return nil
}

func (r *RedisCache) Delete(ctx context.Context, key string) error {
	fullKey := r.config.KeyPrefix + key

	cmd := r.client.B().Del().Key(fullKey).Build()
	if err := r.client.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("redis delete: %w", err)
	}

	return nil
}

func (r *RedisCache) Name() string {
	return r.name
}

func (r *RedisCache) Close() error {
	r.client.Close()
	return nil
}

func (r *RedisCache) BatchGet(ctx context.Context, keys []string) (map[string]interface{}, error) {
	if len(keys) == 0 {
		return make(map[string]interface{}), nil
	}

	cmds := make([]rueidis.Completed, len(keys))
	for i, key := range keys {
		fullKey := r.config.KeyPrefix + key
		cmds[i] = r.client.B().Get().Key(fullKey).Build()
	}

	results := r.client.DoMulti(ctx, cmds...)

	resultMap := make(map[string]interface{}, len(keys))
	var errs []error

	for i, resp := range results {
		if err := resp.Error(); err != nil {
			if !rueidis.IsRedisNil(err) {
				errs = append(errs, fmt.Errorf("key %s: %w", keys[i], err))
			}
			continue
		}

		data, err := resp.AsBytes()
		if err != nil {
			errs = append(errs, fmt.Errorf("key %s: failed to read: %w", keys[i], err))
			continue
		}

		var value interface{}
		if err := json.Unmarshal(data, &value); err != nil {
			errs = append(errs, fmt.Errorf("key %s: failed to unmarshal: %w", keys[i], err))
			continue
		}

		resultMap[keys[i]] = value
	}

	if len(errs) > 0 {
		return resultMap, errors.Join(errs...)
	}

	return resultMap, nil
}

func (r *RedisCache) BatchSet(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	if len(items) == 0 {
		return nil
	}

	cmds := make([]rueidis.Completed, 0, len(items))
	keys := make([]string, 0, len(items))

	for key, value := range items {
		fullKey := r.config.KeyPrefix + key
		keys = append(keys, key)

		data, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("redis batch set: key %s: failed to marshal: %w", key, err)
		}

		cmd := r.client.B().Set().Key(fullKey).Value(string(data)).Ex(ttl).Build()
		cmds = append(cmds, cmd)
	}

	results := r.client.DoMulti(ctx, cmds...)

	var errs []error
	for i, resp := range results {
		if err := resp.Error(); err != nil {
			errs = append(errs, fmt.Errorf("key %s: %w", keys[i], err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (r *RedisCache) BatchDelete(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	fullKeys := make([]string, len(keys))
	for i, key := range keys {
		fullKeys[i] = r.config.KeyPrefix + key
	}

	cmd := r.client.B().Del().Key(fullKeys...).Build()
	if err := r.client.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("redis batch delete: %w", err)
	}

	return nil
}

func (r *RedisCache) Ping(ctx context.Context) error {
	cmd := r.client.B().Ping().Build()
	if err := r.client.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}
	return nil
}

func (r *RedisCache) FlushDB(ctx context.Context) error {
	cmd := r.client.B().Flushdb().Build()
	if err := r.client.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("redis flushdb: %w", err)
	}
	return nil
}

func (r *RedisCache) Keys(ctx context.Context, pattern string) ([]string, error) {
	fullPattern := r.config.KeyPrefix + pattern

	cmd := r.client.B().Keys().Pattern(fullPattern).Build()
	resp := r.client.Do(ctx, cmd)

	if err := resp.Error(); err != nil {
		return nil, fmt.Errorf("redis keys: %w", err)
	}

	keys, err := resp.AsStrSlice()
	if err != nil {
		return nil, fmt.Errorf("redis keys: failed to read response: %w", err)
	}

	prefixLen := len(r.config.KeyPrefix)
	result := make([]string, len(keys))
	for i, key := range keys {
		if len(key) >= prefixLen {
			result[i] = key[prefixLen:]
		} else {
			result[i] = key
		}
	}

	return result, nil
}

func (r *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	fullKey := r.config.KeyPrefix + key

	cmd := r.client.B().Ttl().Key(fullKey).Build()
	resp := r.client.Do(ctx, cmd)

	if err := resp.Error(); err != nil {
		return 0, fmt.Errorf("redis ttl: %w", err)
	}

	seconds, err := resp.AsInt64()
	if err != nil {
		return 0, fmt.Errorf("redis ttl: failed to read response: %w", err)
	}

	if seconds == -2 {
		return 0, cache.ErrCacheMiss
	}

	if seconds == -1 {
		return -1, nil
	}

	return time.Duration(seconds) * time.Second, nil
}

func (r *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	fullKey := r.config.KeyPrefix + key

	cmd := r.client.B().Exists().Key(fullKey).Build()
	resp := r.client.Do(ctx, cmd)

	if err := resp.Error(); err != nil {
		return false, fmt.Errorf("redis exists: %w", err)
	}

	count, err := resp.AsInt64()
	if err != nil {
		return false, fmt.Errorf("redis exists: failed to read response: %w", err)
	}

	return count > 0, nil
}
